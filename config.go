package postgres

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// DefaultConnectTimeout bounds each new PostgreSQL connection attempt.
	DefaultConnectTimeout = 5 * time.Second
	// DefaultAcquireTimeout bounds waiting for a pooled connection.
	DefaultAcquireTimeout = 5 * time.Second
	// DefaultPingTimeout bounds startup and readiness probes.
	DefaultPingTimeout = 2 * time.Second
	// DefaultShutdownTimeout bounds how long Close waits for borrowed connections.
	DefaultShutdownTimeout = 10 * time.Second
	// DefaultConfigResolutionTimeout bounds cooperative resolver preparation.
	DefaultConfigResolutionTimeout = 5 * time.Second
)

// PoolConfig is the native pgxpool configuration type. The alias makes hooks
// explicit while preserving direct access to every pgxpool option.
type PoolConfig = pgxpool.Config

// TLSMode controls whether typed TLS settings override the DSN.
type TLSMode uint8

const (
	// TLSFromDSN preserves pgx parsing of sslmode and related DSN settings.
	TLSFromDSN TLSMode = iota
	// TLSDisable explicitly disables TLS for every configured host.
	TLSDisable
	// TLSRequire requires the supplied tls.Config for every configured host.
	TLSRequire
)

// TLSConfig is an explicit TLS override. TLSRequire copies Config and its
// certificate pools, protocol slices, and certificate bytes before use.
// Callback, private-key, cache, random-source, clock, and writer values remain
// caller-owned collaborators and must be safe for concurrent use.
type TLSConfig struct {
	Mode   TLSMode
	Config *tls.Config
}

// StartupPolicy controls whether Connect proves connectivity before returning.
type StartupPolicy uint8

const (
	// StartupLazy is the default: no startup ping or proactive minimum connections.
	StartupLazy StartupPolicy = iota
	// StartupPing explicitly requests a bounded connectivity check during Connect.
	StartupPing
)

// Config defines safe, finite defaults for constructing a PostgreSQL pool.
// ResolveDSN is required; other zero fields select documented defaults.
// Negative sizes or durations are rejected rather than passed through to pgxpool.
type Config struct {
	DSN string

	// ResolveDSN is required. It owns any environment, filesystem or other
	// acquisition used to resolve the admitted DSN, must cooperate with ctx,
	// and transfers a fresh parser-created native configuration exclusively.
	ResolveDSN func(context.Context, string) (*PoolConfig, error)

	// ConfigResolutionTimeout bounds cooperative preparation; zero uses 5s.
	ConfigResolutionTimeout time.Duration
	// Limits selects finite native credential admission budgets.
	Limits ConfigLimits

	ConnectTimeout  time.Duration
	AcquireTimeout  time.Duration
	PingTimeout     time.Duration
	ShutdownTimeout time.Duration

	MaxConns     int32
	MinConns     int32
	MinIdleConns int32

	MaxConnLifetime       time.Duration
	MaxConnLifetimeJitter time.Duration
	MaxConnIdleTime       time.Duration
	HealthCheckPeriod     time.Duration
	StartupPolicy         StartupPolicy
	TLS                   TLSConfig
	Observer              Observer

	// SessionInit runs for every newly established connection after any native
	// AfterConnect hook. Returning an error rejects that connection.
	SessionInit func(context.Context, *pgx.Conn) error

	// Configure receives the admitted native configuration and preparation
	// deadline after typed options. It must cooperate with ctx and exclusively
	// mutate the transferred configuration without retaining it for later use.
	Configure func(context.Context, *PoolConfig) error
}

// ConfigError reports a field without echoing credentials. Resolver causes are
// withheld; trusted Configure causes remain inspectable.
type ConfigError struct {
	Field   string
	Problem string
	Cause   error
}

// Error implements error.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("postgres: invalid %s: %s", e.Field, e.Problem)
}

// Unwrap exposes a safe underlying cause when one is available.
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// ParseConfig delegates to PrepareConfig with a finite cooperative deadline.
// ResolveDSN is required and owns native parsing and its ambient acquisitions.
// A deadline cannot preempt an uncooperative application callback.
func ParseConfig(input Config) (*PoolConfig, error) {
	return PrepareConfig(context.Background(), input)
}

// PrepareConfig admits the DSN before calling the required application resolver,
// validates native shapes before typed overrides, and revalidates after Configure.
// The resolver transfers a fresh parser-created configuration; TLS data, native
// callbacks, parser allocations and their resource bounds remain application-owned.
// Preparation cooperatively honors ctx and ConfigResolutionTimeout, including
// callback completion checks. It does not recover collaborator panics.
func PrepareConfig(ctx context.Context, input Config) (*PoolConfig, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if input.ResolveDSN == nil {
		return nil, configError("resolver", "is required")
	}
	limits, err := input.Limits.admitted()
	if err != nil {
		return nil, err
	}
	if len(input.DSN) > limits.MaximumDSNBytes {
		return nil, configError("dsn", "exceeds byte limit")
	}
	if input.DSN == "" {
		return nil, configError("dsn", "must not be empty")
	}

	if field, ok := invalidNegativeField(input); ok {
		return nil, configError(field, "must not be negative")
	}
	for _, field := range []struct {
		name  string
		value time.Duration
	}{
		{"acquire_timeout", input.AcquireTimeout},
		{"shutdown_timeout", input.ShutdownTimeout},
	} {
		if field.value > MaximumConfigTimeout {
			return nil, configError(field.name, "is outside the finite policy")
		}
	}
	if input.StartupPolicy > StartupPing {
		return nil, configError("startup_policy", "is not recognized")
	}
	if input.TLS.Mode > TLSRequire {
		return nil, configError("tls.mode", "is not recognized")
	}
	if input.TLS.Mode == TLSRequire && input.TLS.Config == nil {
		return nil, configError("tls.config", "is required when TLS is required")
	}

	resolutionTimeout := valueOrDefault(input.ConfigResolutionTimeout, DefaultConfigResolutionTimeout)
	if resolutionTimeout <= 0 || resolutionTimeout > MaximumConfigTimeout {
		return nil, configError("config_resolution_timeout", "is outside the finite policy")
	}
	ctx, cancel := context.WithTimeout(ctx, resolutionTimeout)
	defer cancel()
	config, err := input.ResolveDSN(ctx, input.DSN)
	if err != nil {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, configError("resolver", "could not resolve configuration")
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	if err := admitNativeShape(config, limits); err != nil {
		return nil, err
	}
	// Native zero is not our policy: choose an explicit finite body allowance
	// before application mutation, then refuse any post-hook relaxation.
	if config.ConnConfig.MaxProtocolMessageBodyLen == 0 {
		config.ConnConfig.MaxProtocolMessageBodyLen = limits.MaximumProtocolMessageBodyBytes
	}

	config.ConnConfig.ConnectTimeout = valueOrDefault(input.ConnectTimeout, DefaultConnectTimeout)
	config.MaxConns = int32OrDefault(input.MaxConns, 10)
	config.MinConns = input.MinConns
	config.MinIdleConns = input.MinIdleConns
	config.MaxConnLifetime = valueOrDefault(input.MaxConnLifetime, time.Hour)
	config.MaxConnLifetimeJitter = valueOrDefault(input.MaxConnLifetimeJitter, 5*time.Minute)
	config.MaxConnIdleTime = valueOrDefault(input.MaxConnIdleTime, 30*time.Minute)
	config.HealthCheckPeriod = valueOrDefault(input.HealthCheckPeriod, time.Minute)
	config.PingTimeout = valueOrDefault(input.PingTimeout, DefaultPingTimeout)
	applyTLSConfig(config, input.TLS)

	if config.MinConns > config.MaxConns {
		return nil, configError("min_conns", "must not exceed max_conns")
	}
	if config.MinIdleConns > config.MaxConns {
		return nil, configError("min_idle_conns", "must not exceed max_conns")
	}

	if input.Configure != nil {
		err := input.Configure(ctx, config)
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		if err != nil {
			return nil, &ConfigError{
				Field:   "configure hook",
				Problem: "returned an error",
				Cause:   err,
			}
		}
	}
	if err := admitNativeShape(config, limits); err != nil {
		return nil, err
	}
	if err := admitNativePolicy(config, input.StartupPolicy); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}

	composeSessionInit(config, input.SessionInit)

	return config, nil
}

func applyTLSConfig(config *PoolConfig, input TLSConfig) {
	switch input.Mode {
	case TLSFromDSN:
		return
	case TLSDisable:
		config.ConnConfig.TLSConfig = nil
		for _, fallback := range config.ConnConfig.Fallbacks {
			fallback.TLSConfig = nil
		}
	case TLSRequire:
		config.ConnConfig.TLSConfig = tlsConfigForHost(input.Config, config.ConnConfig.Host)
		for _, fallback := range config.ConnConfig.Fallbacks {
			fallback.TLSConfig = tlsConfigForHost(input.Config, fallback.Host)
		}
	}
}

func tlsConfigForHost(input *tls.Config, host string) *tls.Config {
	config := cloneTLSConfig(input)
	if config.ServerName == "" {
		config.ServerName = host
	}

	return config
}

func cloneTLSConfig(input *tls.Config) *tls.Config {
	config := input.Clone()
	if config.RootCAs != nil {
		config.RootCAs = config.RootCAs.Clone()
	}
	if config.ClientCAs != nil {
		config.ClientCAs = config.ClientCAs.Clone()
	}
	config.NextProtos = append([]string(nil), config.NextProtos...)
	config.CipherSuites = append([]uint16(nil), config.CipherSuites...)
	config.CurvePreferences = append([]tls.CurveID(nil), config.CurvePreferences...)
	config.EncryptedClientHelloConfigList = append(
		[]byte(nil),
		config.EncryptedClientHelloConfigList...,
	)
	config.Certificates = append([]tls.Certificate(nil), config.Certificates...)
	for index := range config.Certificates {
		certificate := &config.Certificates[index]
		certificate.Certificate = cloneByteSlices(certificate.Certificate)
		certificate.SupportedSignatureAlgorithms = append(
			[]tls.SignatureScheme(nil),
			certificate.SupportedSignatureAlgorithms...,
		)
		certificate.OCSPStaple = append([]byte(nil), certificate.OCSPStaple...)
		certificate.SignedCertificateTimestamps = cloneByteSlices(
			certificate.SignedCertificateTimestamps,
		)
		certificate.Leaf = nil
	}

	return config
}

func cloneByteSlices(input [][]byte) [][]byte {
	result := make([][]byte, len(input))
	for index := range input {
		result[index] = append([]byte(nil), input[index]...)
	}

	return result
}

func composeSessionInit(config *PoolConfig, sessionInit func(context.Context, *pgx.Conn) error) {
	if sessionInit == nil {
		return
	}

	nativeAfterConnect := config.AfterConnect
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if nativeAfterConnect != nil {
			if err := nativeAfterConnect(ctx, conn); err != nil {
				return err
			}
		}
		if err := sessionInit(ctx, conn); err != nil {
			return fmt.Errorf("postgres: initialize session: %w", err)
		}

		return nil
	}
}

func configError(field, problem string) error {
	return &ConfigError{Field: field, Problem: problem}
}

func valueOrDefault(value, defaultValue time.Duration) time.Duration {
	if value == 0 {
		return defaultValue
	}

	return value
}

func int32OrDefault(value, defaultValue int32) int32 {
	if value == 0 {
		return defaultValue
	}

	return value
}

func invalidNegativeField(config Config) (string, bool) {
	fields := []struct {
		name  string
		value int64
	}{
		{name: "connect_timeout", value: int64(config.ConnectTimeout)},
		{name: "config_resolution_timeout", value: int64(config.ConfigResolutionTimeout)},
		{name: "acquire_timeout", value: int64(config.AcquireTimeout)},
		{name: "ping_timeout", value: int64(config.PingTimeout)},
		{name: "shutdown_timeout", value: int64(config.ShutdownTimeout)},
		{name: "max_conns", value: int64(config.MaxConns)},
		{name: "min_conns", value: int64(config.MinConns)},
		{name: "min_idle_conns", value: int64(config.MinIdleConns)},
		{name: "max_conn_lifetime", value: int64(config.MaxConnLifetime)},
		{name: "max_conn_lifetime_jitter", value: int64(config.MaxConnLifetimeJitter)},
		{name: "max_conn_idle_time", value: int64(config.MaxConnIdleTime)},
		{name: "health_check_period", value: int64(config.HealthCheckPeriod)},
	}

	for _, field := range fields {
		if field.value < 0 {
			return field.name, true
		}
	}

	return "", false
}
