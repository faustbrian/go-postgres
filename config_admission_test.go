package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These synthetic configurations exercise package admission only. They do not
// have pgx's private parser provenance and must never reach the native factory.
func admittedNative() *PoolConfig {
	return &PoolConfig{ConnConfig: &pgx.ConnConfig{Config: pgconn.Config{
		Host: "db", Database: "app", User: "user", Password: "key",
	}}}
}

func admittedInput() Config {
	return Config{DSN: "db", ResolveDSN: func(context.Context, string) (*PoolConfig, error) {
		return admittedNative(), nil
	}}
}

func requireConfigField(t *testing.T, result *PoolConfig, err error, field string) {
	t.Helper()
	var detail *ConfigError
	if result != nil || !errors.As(err, &detail) || detail.Field != field || detail.Cause != nil {
		t.Fatal("configuration admission did not preserve its safe refusal category")
	}
}

func TestConfigAdmissionDSN(t *testing.T) {
	input := admittedInput()
	input.Limits.MaximumDSNBytes = 2
	if _, err := PrepareConfig(context.Background(), input); err != nil {
		t.Fatal("inclusive DSN budget refused")
	}
	called := false
	input.DSN = "dbx"
	input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
		called = true
		return admittedNative(), nil
	}
	result, err := PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "dsn")
	if called {
		t.Fatal("oversize DSN reached resolver")
	}
}

func TestConfigAdmissionResolutionTimeoutCeiling(t *testing.T) {
	input := admittedInput()
	input.ConfigResolutionTimeout = time.Hour
	resolved := false
	input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
		resolved = true
		return admittedNative(), nil
	}
	native, err := PrepareConfig(context.Background(), input)
	if err != nil || native == nil || !resolved {
		t.Fatal("inclusive resolution timeout refused")
	}
	resolved = false
	input.ConfigResolutionTimeout = time.Hour + time.Nanosecond
	native, err = PrepareConfig(context.Background(), input)
	requireConfigField(t, native, err, "config_resolution_timeout")
	if resolved || err.Error() != "postgres: invalid config_resolution_timeout: is outside the finite policy" {
		t.Fatal("resolution timeout refusal crossed resolver or diagnostic boundary")
	}
}

func TestConfigAdmissionRuntimeParameterStringBudgets(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  string
		budget int
	}{
		// The native scalar fields use 12 bytes; application_name uses 16.
		// With an empty value, reducing 28 to 27 refuses the key itself.
		{"key", "", 28},
		// The key fits at 30, but the three-byte value requires 31 total.
		{"value", "app", 31},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumNativeStringBytes = test.budget
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				native.ConnConfig.RuntimeParams = map[string]string{"application_name": test.value}
				return native, nil
			}
			configured := false
			input.Configure = func(context.Context, *PoolConfig) error { configured = true; return nil }
			constructed := false
			backend := &admissionBackend{}
			factory := func(_ context.Context, native *PoolConfig) (*pgxpool.Pool, poolBackend, error) {
				constructed = true
				value, present := native.ConnConfig.RuntimeParams["application_name"]
				if !present || value != test.value || len(native.ConnConfig.RuntimeParams) != 1 {
					t.Fatal("admitted runtime parameter changed before factory")
				}
				return nil, backend, nil
			}
			pool, err := connect(context.Background(), input, factory)
			if err != nil || pool == nil || !configured || !constructed {
				t.Fatal("inclusive runtime string budget refused")
			}
			owned := pool
			t.Cleanup(func() {
				if err := owned.Shutdown(context.Background()); err != nil || backend.closes != 1 {
					t.Error("owned fake backend cleanup failed")
				}
			})
			configured, constructed = false, false
			input.Limits.MaximumNativeStringBytes--
			pool, err = connect(context.Background(), input, factory)
			var detail *ConfigError
			if pool != nil || !errors.As(err, &detail) || detail.Field != "native_config" || detail.Cause != nil || configured || constructed {
				t.Fatal("runtime string refusal crossed hook or factory boundary")
			}
			if err.Error() != "postgres: invalid native_config: exceeds or violates admission policy" {
				t.Fatal("runtime string refusal exposed a nonconstant diagnostic")
			}
		})
	}
}

func TestConfigAdmissionNativeBudgets(t *testing.T) {
	for _, shape := range []string{"strings", "fallbacks", "params"} {
		t.Run(shape, func(t *testing.T) {
			input := admittedInput()
			input.Limits = ConfigLimits{MaximumNativeStringBytes: 12, MaximumFallbacks: 1, MaximumRuntimeParams: 1}
			makeNative := func(over bool) *PoolConfig {
				native := admittedNative()
				switch shape {
				case "strings":
					if over {
						native.ConnConfig.Password = "keys"
					}
				case "fallbacks":
					native.ConnConfig.Password = ""
					native.ConnConfig.Fallbacks = []*pgconn.FallbackConfig{{Host: "a"}}
					if over {
						native.ConnConfig.Fallbacks = append(native.ConnConfig.Fallbacks, &pgconn.FallbackConfig{Host: "b"})
					}
				case "params":
					native.ConnConfig.Password = ""
					native.ConnConfig.RuntimeParams = map[string]string{"a": "b"}
					if over {
						native.ConnConfig.RuntimeParams["c"] = "d"
					}
				}
				return native
			}
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) { return makeNative(false), nil }
			if _, err := PrepareConfig(context.Background(), input); err != nil {
				t.Fatal("inclusive native budget refused")
			}
			configured := false
			input.Configure = func(_ context.Context, _ *PoolConfig) error { configured = true; return nil }
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) { return makeNative(true), nil }
			result, err := PrepareConfig(context.Background(), input)
			requireConfigField(t, result, err, "native_config")
			if configured {
				t.Fatal("refused native shape reached configuration hook")
			}
		})
	}
}

func TestConfigAdmissionContextAndResolverPrivacy(t *testing.T) {
	input := admittedInput()
	result, err := PrepareConfig(nil, input)
	if result != nil || !errors.Is(err, ErrContextRequired) {
		t.Fatal("nil context admitted")
	}
	cause := errors.New("ordinary cancellation")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	called := false
	input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) { called = true; return admittedNative(), nil }
	result, err = PrepareConfig(ctx, input)
	if result != nil || !errors.Is(err, cause) || called {
		t.Fatal("canceled preparation crossed resolver boundary")
	}
	ctx, cancel = context.WithCancelCause(context.Background())
	input.ResolveDSN = func(received context.Context, _ string) (*PoolConfig, error) {
		if _, ok := received.Deadline(); !ok {
			t.Fatal("resolver has no deadline")
		}
		cancel(cause)
		return admittedNative(), nil
	}
	result, err = PrepareConfig(ctx, input)
	if result != nil || !errors.Is(err, cause) {
		t.Fatal("resolver cancellation admitted")
	}
	input = admittedInput()
	upstream := errors.New("ordinary upstream refusal")
	input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) { return nil, upstream }
	result, err = PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "resolver")
	if errors.Is(err, upstream) || err.Error() != "postgres: invalid resolver: could not resolve configuration" {
		t.Fatal("resolver cause escaped privacy boundary")
	}
	input = admittedInput()
	ctx, cancel = context.WithCancelCause(context.Background())
	input.Configure = func(_ context.Context, _ *PoolConfig) error { cancel(cause); return nil }
	result, err = PrepareConfig(ctx, input)
	if result != nil || !errors.Is(err, cause) {
		t.Fatal("hook cancellation admitted")
	}
}

func TestConfigAdmissionFinalHookAndLazy(t *testing.T) {
	for _, field := range []string{"min_conns", "min_idle_conns", "max_conns", "connect_timeout", "ping_timeout", "health_check_period", "max_conn_lifetime", "max_conn_idle_time", "native_config"} {
		t.Run(field, func(t *testing.T) {
			input := admittedInput()
			input.Configure = func(_ context.Context, native *PoolConfig) error {
				switch field {
				case "min_conns":
					native.MinConns = 1
				case "min_idle_conns":
					native.MinIdleConns = 1
				case "max_conns":
					native.MaxConns = 0
				case "connect_timeout":
					native.ConnConfig.ConnectTimeout = 0
				case "ping_timeout":
					native.PingTimeout = 0
				case "health_check_period":
					native.HealthCheckPeriod = 0
				case "max_conn_lifetime":
					native.MaxConnLifetime = 0
				case "max_conn_idle_time":
					native.MaxConnIdleTime = 0
				case "native_config":
					native.ConnConfig.Password = "keys"
				}
				return nil
			}
			input.Limits.MaximumNativeStringBytes = 12
			result, err := PrepareConfig(context.Background(), input)
			requireConfigField(t, result, err, field)
		})
	}
	input := admittedInput()
	input.ConfigResolutionTimeout = time.Second
	if _, err := ParseConfig(input); err != nil {
		t.Fatal("synchronous preparation refused valid input")
	}
	if StartupLazy != 0 {
		t.Fatal("zero startup policy is not lazy")
	}
}

func TestConfigAdmissionShapeAndConfigurationRefusal(t *testing.T) {
	for _, shape := range []string{"nil_config", "nil_connection", "nil_fallback"} {
		t.Run(shape, func(t *testing.T) {
			input := admittedInput()
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				switch shape {
				case "nil_config":
					return nil, nil
				case "nil_connection":
					native.ConnConfig = nil
				case "nil_fallback":
					native.ConnConfig.Fallbacks = []*pgconn.FallbackConfig{nil}
				}
				return native, nil
			}
			result, err := PrepareConfig(context.Background(), input)
			requireConfigField(t, result, err, "native_config")
		})
	}
	input := admittedInput()
	input.Limits.MaximumDSNBytes = -1
	result, err := PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "limits")
	input = admittedInput()
	input.ConfigResolutionTimeout = -time.Second
	result, err = PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "config_resolution_timeout")
	input = admittedInput()
	var resolutionContext context.Context
	input.ResolveDSN = func(ctx context.Context, _ string) (*PoolConfig, error) {
		resolutionContext = ctx
		return admittedNative(), nil
	}
	input.Configure = func(ctx context.Context, _ *PoolConfig) error {
		if _, ok := ctx.Deadline(); !ok || ctx != resolutionContext {
			t.Fatal("hook did not receive preparation deadline")
		}
		return nil
	}
	if _, err := PrepareConfig(context.Background(), input); err != nil {
		t.Fatal("context-aware hook refused")
	}
}

type admissionBackend struct {
	pings, closes int
	pingError     error
}

func (b *admissionBackend) Acquire(context.Context) (*pgxpool.Conn, error) { return nil, ErrPoolClosed }
func (b *admissionBackend) Ping(context.Context) error                     { b.pings++; return b.pingError }
func (b *admissionBackend) Stats() Stats                                   { return Stats{} }
func (b *admissionBackend) Close()                                         { b.closes++ }

func TestConfigAdmissionConnectFactory(t *testing.T) {
	backend := &admissionBackend{}
	constructs := 0
	factory := func(_ context.Context, native *PoolConfig) (*pgxpool.Pool, poolBackend, error) {
		constructs++
		if native.MaxConns != 10 {
			t.Fatal("factory received unprepared configuration")
		}
		return nil, backend, nil
	}
	input := admittedInput()
	pool, err := connect(context.Background(), input, factory)
	if err != nil || pool == nil || constructs != 1 || backend.pings != 0 {
		t.Fatal("default startup performed readiness or failed construction")
	}
	if err := pool.Shutdown(context.Background()); err != nil || backend.closes != 1 {
		t.Fatal("lazy pool did not close owned backend")
	}
	backend = &admissionBackend{pingError: errors.New("ordinary readiness failure")}
	input.StartupPolicy = StartupPing
	pool, err = connect(context.Background(), input, factory)
	if pool != nil || !errors.Is(err, backend.pingError) || backend.pings != 1 || backend.closes != 1 {
		t.Fatal("explicit readiness did not preserve error and cleanup")
	}
	ctx, cancel := context.WithCancel(context.Background())
	input = admittedInput()
	input.Configure = func(context.Context, *PoolConfig) error { cancel(); return nil }
	before := constructs
	pool, err = connect(ctx, input, factory)
	if pool != nil || !errors.Is(err, context.Canceled) || constructs != before {
		t.Fatal("cancellation reached factory")
	}
	input = admittedInput()
	input.Configure = func(_ context.Context, native *PoolConfig) error { native.MinIdleConns = 1; return nil }
	pool, err = connect(context.Background(), input, factory)
	var detail *ConfigError
	if pool != nil || !errors.As(err, &detail) || detail.Field != "min_idle_conns" || constructs != before {
		t.Fatal("lazy minimum reached factory")
	}
}

func TestConfigAdmissionFactoryFailurePreservesCause(t *testing.T) {
	cause := errors.New("ordinary construction failure")
	pool, err := connect(context.Background(), admittedInput(), func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolBackend, error) {
		return nil, nil, cause
	})
	if pool != nil || !errors.Is(err, cause) || !errors.Is(errors.Unwrap(err), cause) {
		t.Fatal("factory failure lost its cause or returned a pool")
	}
	if err.Error() != "postgres: startup connectivity check failed" {
		t.Fatal("factory failure exposed an unbounded diagnostic")
	}
}

func TestOpenNativePoolRejectsInvalidMaximumHosted(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("native adapter construction refusal runs only in hosted CI")
	}

	// This is the internal adapter's native error contract. Public Connect
	// refuses this maximum during admission before calling the adapter.
	native, err := pgxpool.ParseConfig("host=database.example port=5432 user=example database=example sslmode=disable")
	if err != nil {
		t.Fatal("ordinary native configuration refused")
	}
	native.MaxConns = 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	raw, backend, err := openNativePool(ctx, native)
	if raw != nil {
		raw.Close()
	}
	if raw != nil || backend != nil || err == nil {
		t.Fatal("invalid native maximum did not refuse construction")
	}
	if err.Error() != "MaxSize must be >= 1" {
		t.Fatal("native construction refusal lost its category")
	}
}

func TestConfigAdmissionProtocolMessageBudget(t *testing.T) {
	input := admittedInput()
	input.Limits.MaximumProtocolMessageBodyBytes = 3
	configured := false
	input.Configure = func(_ context.Context, native *PoolConfig) error {
		configured = true
		if native.ConnConfig.MaxProtocolMessageBodyLen != 3 {
			t.Fatal("native zero did not select finite message budget before hook")
		}
		return nil
	}
	native, err := PrepareConfig(context.Background(), input)
	if err != nil || native == nil || native.ConnConfig.MaxProtocolMessageBodyLen != 3 || !configured {
		t.Fatal("finite message default not retained")
	}
	for _, value := range []int{3, 4, -1} {
		t.Run("resolver", func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumProtocolMessageBodyBytes = 3
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				native.ConnConfig.MaxProtocolMessageBodyLen = value
				return native, nil
			}
			configured := false
			input.Configure = func(context.Context, *PoolConfig) error { configured = true; return nil }
			native, err := PrepareConfig(context.Background(), input)
			if value == 3 {
				if err != nil || native == nil || native.ConnConfig.MaxProtocolMessageBodyLen != value || !configured {
					t.Fatal("inclusive message allowance refused")
				}
			} else {
				requireConfigField(t, native, err, "native_config")
				if configured {
					t.Fatal("invalid message allowance reached hook")
				}
			}
		})
	}
}

func TestConfigAdmissionProtocolHookRefusal(t *testing.T) {
	for _, value := range []int{0, -1, 4} {
		t.Run("hook", func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumProtocolMessageBodyBytes = 3
			input.Configure = func(_ context.Context, native *PoolConfig) error {
				native.ConnConfig.MaxProtocolMessageBodyLen = value
				return nil
			}
			constructed := false
			pool, err := connect(context.Background(), input, func(context.Context, *pgxpool.Config) (*pgxpool.Pool, poolBackend, error) {
				constructed = true
				return nil, &admissionBackend{}, nil
			})
			var detail *ConfigError
			if pool != nil || !errors.As(err, &detail) || detail.Field != "native_config" || detail.Cause != nil || constructed {
				t.Fatal("hook relaxed finite message policy before factory")
			}
		})
	}
}

func TestConfigAdmissionWrapperTimeoutCeilings(t *testing.T) {
	for _, field := range []string{"acquire_timeout", "shutdown_timeout"} {
		t.Run(field, func(t *testing.T) {
			input := admittedInput()
			resolved := false
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				resolved = true
				return admittedNative(), nil
			}
			if field == "acquire_timeout" {
				input.AcquireTimeout = MaximumConfigTimeout
			} else {
				input.ShutdownTimeout = MaximumConfigTimeout
			}
			native, err := PrepareConfig(context.Background(), input)
			if err != nil || native == nil || !resolved {
				t.Fatal("inclusive wrapper timeout ceiling refused")
			}
			resolved = false
			if field == "acquire_timeout" {
				input.AcquireTimeout++
			} else {
				input.ShutdownTimeout++
			}
			native, err = PrepareConfig(context.Background(), input)
			requireConfigField(t, native, err, field)
			if resolved || err.Error() != "postgres: invalid "+field+": is outside the finite policy" {
				t.Fatal("wrapper timeout refusal crossed resolver or diagnostic boundary")
			}
		})
	}
}

func TestConfigAdmissionCanceledResolverErrorPrecedence(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	cause := errors.New("ordinary caller cancellation")
	upstream := errors.New("ordinary resolver refusal")
	input := admittedInput()
	input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
		cancel(cause)
		return nil, upstream
	}
	configured := false
	input.Configure = func(context.Context, *PoolConfig) error { configured = true; return nil }
	native, err := PrepareConfig(ctx, input)
	var detail *ConfigError
	if native != nil || !errors.Is(err, cause) || errors.Is(err, upstream) || errors.As(err, &detail) || configured {
		t.Fatal("canceled resolver error lost caller cause or continued preparation")
	}
}

func TestConfigAdmissionHookJitterBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		jitter   time.Duration
		accepted bool
	}{
		{"zero", 0, true},
		{"inclusive", time.Second, true},
		{"negative", -time.Nanosecond, false},
		{"one_over", time.Second + time.Nanosecond, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := admittedInput()
			input.Configure = func(_ context.Context, native *PoolConfig) error {
				native.MaxConnLifetime = time.Second
				native.MaxConnLifetimeJitter = test.jitter
				return nil
			}
			constructed := false
			pool, err := connect(context.Background(), input, func(_ context.Context, native *pgxpool.Config) (*pgxpool.Pool, poolBackend, error) {
				constructed = true
				if native.MaxConnLifetime != time.Second || native.MaxConnLifetimeJitter != test.jitter {
					t.Fatal("admitted jitter changed before factory")
				}
				return nil, &admissionBackend{}, nil
			})
			if pool != nil {
				t.Cleanup(func() {
					if err := pool.Shutdown(context.Background()); err != nil {
						t.Error("owned fake backend cleanup failed")
					}
				})
			}
			if test.accepted {
				if err != nil || pool == nil || !constructed {
					t.Fatal("inclusive hook jitter refused")
				}
				return
			}
			var detail *ConfigError
			if pool != nil || !errors.As(err, &detail) || detail.Field != "max_conn_lifetime_jitter" || detail.Cause != nil || constructed {
				t.Fatal("out-of-policy hook jitter reached factory or lost refusal category")
			}
			if err.Error() != "postgres: invalid max_conn_lifetime_jitter: must be within the connection lifetime" {
				t.Fatal("jitter refusal exposed a nonconstant diagnostic")
			}
		})
	}
}

func TestConfigAdmissionAdditionalNativeFields(t *testing.T) {
	for _, field := range []string{"kerberos_service", "kerberos_spn", "ssl_negotiation", "min_protocol", "max_protocol", "channel_binding", "require_auth", "statement_cache", "description_cache"} {
		t.Run(field, func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumNativeStringBytes = 12
			input.Limits.MaximumStatementCacheEntries = 1
			input.Limits.MaximumDescriptionCacheEntries = 1
			set := func(native *PoolConfig, over bool) {
				value := ""
				if over {
					value = "postgres"
					switch field {
					case "min_protocol", "max_protocol":
						value = "3.0"
					case "channel_binding":
						value = "prefer"
					case "require_auth":
						value = "scram-sha-256"
					}
				}
				switch field {
				case "kerberos_service":
					native.ConnConfig.KerberosSrvName = value
				case "kerberos_spn":
					native.ConnConfig.KerberosSpn = value
				case "ssl_negotiation":
					native.ConnConfig.SSLNegotiation = value
				case "min_protocol":
					native.ConnConfig.MinProtocolVersion = value
				case "max_protocol":
					native.ConnConfig.MaxProtocolVersion = value
				case "channel_binding":
					native.ConnConfig.ChannelBinding = value
				case "require_auth":
					native.ConnConfig.RequireAuth = value
				case "statement_cache":
					native.ConnConfig.StatementCacheCapacity = 1
					if over {
						native.ConnConfig.StatementCacheCapacity = 2
					}
				case "description_cache":
					native.ConnConfig.DescriptionCacheCapacity = 1
					if over {
						native.ConnConfig.DescriptionCacheCapacity = 2
					}
				}
			}
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				set(native, false)
				return native, nil
			}
			if _, err := PrepareConfig(context.Background(), input); err != nil {
				t.Fatal("inclusive native shape refused")
			}
			configured := false
			input.Configure = func(context.Context, *PoolConfig) error { configured = true; return nil }
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				set(native, true)
				return native, nil
			}
			result, err := PrepareConfig(context.Background(), input)
			requireConfigField(t, result, err, "native_config")
			if configured {
				t.Fatal("refused native field reached mutation hook")
			}
		})
	}
}
