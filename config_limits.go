package postgres

import "time"

const (
	// DefaultMaximumDSNBytes bounds input before resolver acquisition.
	DefaultMaximumDSNBytes = 8 << 10
	// DefaultMaximumFallbacks bounds native fallback host entries.
	DefaultMaximumFallbacks = 16
	// DefaultMaximumRuntimeParams bounds native session parameter entries.
	DefaultMaximumRuntimeParams = 128
	// DefaultMaximumNativeStringBytes bounds aggregate native credential strings.
	DefaultMaximumNativeStringBytes = 64 << 10
	// DefaultMaximumCacheEntries bounds each native statement/description cache.
	DefaultMaximumCacheEntries = 1024
	// DefaultMaximumProtocolMessageBodyBytes bounds each native wire-message body.
	DefaultMaximumProtocolMessageBodyBytes = 8 << 20
	// MaximumProtocolMessageBodyCeiling is the largest wire-message body an
	// application may opt into. It equals PostgreSQL's PG_LARGE_MESSAGE_LIMIT
	// (src/include/libpq/libpq.h), which pgx also enforces as maxMessageBodyLen
	// (pgproto3/pgproto3.go); no larger message can be valid.
	MaximumProtocolMessageBodyCeiling = 0x3fffffff - 1
	// MaximumPoolConnections is the finite pool-size ceiling.
	MaximumPoolConnections = 1024
	// MaximumConfigTimeout bounds preparation and connection/operation timeouts.
	MaximumConfigTimeout = time.Hour
	// MaximumPoolLifetime caps native lifetime, idle and maintenance settings.
	MaximumPoolLifetime = 30 * 24 * time.Hour
)

// ConfigLimits controls package-owned admission. Zero selects finite defaults;
// positive values may only reduce the defaults, except
// MaximumProtocolMessageBodyBytes, which may also be raised up to
// MaximumProtocolMessageBodyCeiling for applications that read rows larger
// than the default. The application must bound opaque TLS data and resource
// use by trusted callbacks separately.
type ConfigLimits struct {
	// MaximumDSNBytes admits the input before application resolution.
	MaximumDSNBytes int
	// MaximumFallbacks admits the native fallback slice before traversal.
	MaximumFallbacks int
	// MaximumRuntimeParams admits the native map before traversal.
	MaximumRuntimeParams int
	// MaximumNativeStringBytes charges every native string occurrence, including
	// the retained original connection string, all scalar settings, fallback
	// hosts, and runtime parameter keys/values. Opaque TLS data is not charged.
	MaximumNativeStringBytes int
	// MaximumStatementCacheEntries bounds the native prepared statement cache.
	MaximumStatementCacheEntries int
	// MaximumDescriptionCacheEntries bounds the native description cache.
	MaximumDescriptionCacheEntries int
	// MaximumProtocolMessageBodyBytes bounds each native wire-message body, and
	// therefore the largest row a query can return. Raising it lets one
	// connection buffer that many bytes, so size the pool accordingly.
	MaximumProtocolMessageBodyBytes int
}

func (limits ConfigLimits) admitted() (ConfigLimits, error) {
	fields := []struct {
		value        *int
		defaultValue int
		ceiling      int
	}{
		{&limits.MaximumDSNBytes, DefaultMaximumDSNBytes, DefaultMaximumDSNBytes},
		{&limits.MaximumFallbacks, DefaultMaximumFallbacks, DefaultMaximumFallbacks},
		{&limits.MaximumRuntimeParams, DefaultMaximumRuntimeParams, DefaultMaximumRuntimeParams},
		{&limits.MaximumNativeStringBytes, DefaultMaximumNativeStringBytes, DefaultMaximumNativeStringBytes},
		{&limits.MaximumStatementCacheEntries, DefaultMaximumCacheEntries, DefaultMaximumCacheEntries},
		{&limits.MaximumDescriptionCacheEntries, DefaultMaximumCacheEntries, DefaultMaximumCacheEntries},
		{&limits.MaximumProtocolMessageBodyBytes, DefaultMaximumProtocolMessageBodyBytes, MaximumProtocolMessageBodyCeiling},
	}
	for _, field := range fields {
		if *field.value < 0 || *field.value > field.ceiling {
			return ConfigLimits{}, configError("limits", "must be zero or within its ceiling")
		}
		if *field.value == 0 {
			*field.value = field.defaultValue
		}
	}
	return limits, nil
}

// admitNativeShape bounds collection shapes before iteration and charges native
// strings, without traversing opaque TLS or invoking application collaborators.
func admitNativeShape(config *PoolConfig, limits ConfigLimits) error {
	refusal := func() error { return configError("native_config", "exceeds or violates admission policy") }
	if config == nil || config.ConnConfig == nil {
		return refusal()
	}
	conn := config.ConnConfig
	if conn.MaxProtocolMessageBodyLen < 0 || conn.MaxProtocolMessageBodyLen > limits.MaximumProtocolMessageBodyBytes {
		return refusal()
	}
	if conn.StatementCacheCapacity < 0 || conn.StatementCacheCapacity > limits.MaximumStatementCacheEntries || conn.DescriptionCacheCapacity < 0 || conn.DescriptionCacheCapacity > limits.MaximumDescriptionCacheEntries {
		return refusal()
	}
	if len(conn.Fallbacks) > limits.MaximumFallbacks || len(conn.RuntimeParams) > limits.MaximumRuntimeParams {
		return refusal()
	}
	remaining := limits.MaximumNativeStringBytes
	charge := func(value string) bool {
		if len(value) > remaining {
			return false
		}
		remaining -= len(value)
		return true
	}
	if !charge(conn.Host) || !charge(conn.Database) || !charge(conn.User) || !charge(conn.Password) || !charge(conn.ConnString()) {
		return refusal()
	}
	if !charge(conn.KerberosSrvName) || !charge(conn.KerberosSpn) || !charge(conn.SSLNegotiation) || !charge(conn.MinProtocolVersion) || !charge(conn.MaxProtocolVersion) || !charge(conn.ChannelBinding) || !charge(conn.RequireAuth) {
		return refusal()
	}
	for _, fallback := range conn.Fallbacks {
		if fallback == nil || !charge(fallback.Host) {
			return refusal()
		}
	}
	for key, value := range conn.RuntimeParams {
		if !charge(key) || !charge(value) {
			return refusal()
		}
	}
	return nil
}

func admitNativePolicy(config *PoolConfig, startup StartupPolicy) error {
	if config.ConnConfig.MaxProtocolMessageBodyLen <= 0 {
		return configError("native_config", "requires a finite positive message budget")
	}
	if config.MaxConns <= 0 || config.MaxConns > MaximumPoolConnections {
		return configError("max_conns", "is outside the finite policy")
	}
	if config.MinConns < 0 || config.MinConns > config.MaxConns || (startup == StartupLazy && config.MinConns != 0) {
		return configError("min_conns", "is incompatible with pool policy")
	}
	if config.MinIdleConns < 0 || config.MinIdleConns > config.MaxConns || (startup == StartupLazy && config.MinIdleConns != 0) {
		return configError("min_idle_conns", "is incompatible with pool policy")
	}
	for _, field := range []struct {
		name           string
		value, ceiling time.Duration
	}{
		{"connect_timeout", config.ConnConfig.ConnectTimeout, MaximumConfigTimeout},
		{"ping_timeout", config.PingTimeout, MaximumConfigTimeout},
		{"max_conn_lifetime", config.MaxConnLifetime, MaximumPoolLifetime},
		{"max_conn_idle_time", config.MaxConnIdleTime, MaximumPoolLifetime},
		{"health_check_period", config.HealthCheckPeriod, MaximumPoolLifetime},
	} {
		if field.value <= 0 || field.value > field.ceiling {
			return configError(field.name, "is outside the finite policy")
		}
	}
	if config.MaxConnLifetimeJitter < 0 || config.MaxConnLifetimeJitter > config.MaxConnLifetime {
		return configError("max_conn_lifetime_jitter", "must be within the connection lifetime")
	}
	return nil
}
