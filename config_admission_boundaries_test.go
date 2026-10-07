package postgres

import (
	"context"
	"testing"
	"time"
)

// Explicit ceilings have the same inclusive meaning as defaulted ceilings.
func TestConfigAdmissionExplicitLimitCeilings(t *testing.T) {
	for _, test := range []struct {
		name    string
		ceiling int
		set     func(*ConfigLimits, int)
	}{
		{"dsn", DefaultMaximumDSNBytes, func(l *ConfigLimits, n int) { l.MaximumDSNBytes = n }},
		{"fallbacks", DefaultMaximumFallbacks, func(l *ConfigLimits, n int) { l.MaximumFallbacks = n }},
		{"params", DefaultMaximumRuntimeParams, func(l *ConfigLimits, n int) { l.MaximumRuntimeParams = n }},
		{"strings", DefaultMaximumNativeStringBytes, func(l *ConfigLimits, n int) { l.MaximumNativeStringBytes = n }},
		{"statement_cache", DefaultMaximumCacheEntries, func(l *ConfigLimits, n int) { l.MaximumStatementCacheEntries = n }},
		{"description_cache", DefaultMaximumCacheEntries, func(l *ConfigLimits, n int) { l.MaximumDescriptionCacheEntries = n }},
		{"message", DefaultMaximumProtocolMessageBodyBytes, func(l *ConfigLimits, n int) { l.MaximumProtocolMessageBodyBytes = n }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := admittedInput()
			test.set(&input.Limits, test.ceiling)
			if result, err := PrepareConfig(context.Background(), input); err != nil || result == nil {
				t.Fatal("explicit inclusive limit refused")
			}
			test.set(&input.Limits, test.ceiling+1)
			result, err := PrepareConfig(context.Background(), input)
			requireConfigField(t, result, err, "limits")
		})
	}
}

func TestConfigAdmissionPoolMaximumCeiling(t *testing.T) {
	input := admittedInput()
	input.MaxConns = MaximumPoolConnections
	if result, err := PrepareConfig(context.Background(), input); err != nil || result == nil || result.MaxConns != MaximumPoolConnections {
		t.Fatal("inclusive pool maximum refused or changed")
	}
	input.MaxConns++
	result, err := PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "max_conns")
}

func TestConfigAdmissionPoolCountBoundaries(t *testing.T) {
	for _, field := range []struct {
		name string
		set  func(*PoolConfig, int32)
	}{
		{"min_conns", func(p *PoolConfig, n int32) { p.MinConns = n }},
		{"min_idle_conns", func(p *PoolConfig, n int32) { p.MinIdleConns = n }},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, value := range []int32{-1, 11, 10} {
				input := admittedInput()
				// Lazy startup has an independent zero-minimum restriction that
				// must not mask the ordinary post-hook count bounds here.
				input.StartupPolicy = StartupPing
				input.Configure = func(_ context.Context, p *PoolConfig) error {
					field.set(p, value)
					return nil
				}
				result, err := PrepareConfig(context.Background(), input)
				if value == 10 {
					if err != nil || result == nil || result.MinConns != 10 && result.MinIdleConns != 10 {
						t.Fatal("inclusive post-hook minimum refused or changed")
					}
				} else {
					requireConfigField(t, result, err, field.name)
				}
			}
		})
	}
}

func TestConfigAdmissionNativeDurationCeilings(t *testing.T) {
	for _, field := range []struct {
		name    string
		ceiling time.Duration
		set     func(*PoolConfig, time.Duration)
	}{
		{"connect_timeout", MaximumConfigTimeout, func(p *PoolConfig, d time.Duration) { p.ConnConfig.ConnectTimeout = d }},
		{"ping_timeout", MaximumConfigTimeout, func(p *PoolConfig, d time.Duration) { p.PingTimeout = d }},
		{"max_conn_lifetime", MaximumPoolLifetime, func(p *PoolConfig, d time.Duration) { p.MaxConnLifetime = d }},
		{"max_conn_idle_time", MaximumPoolLifetime, func(p *PoolConfig, d time.Duration) { p.MaxConnIdleTime = d }},
		{"health_check_period", MaximumPoolLifetime, func(p *PoolConfig, d time.Duration) { p.HealthCheckPeriod = d }},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, value := range []time.Duration{field.ceiling, field.ceiling + time.Nanosecond} {
				input := admittedInput()
				input.Configure = func(_ context.Context, p *PoolConfig) error {
					field.set(p, value)
					return nil
				}
				result, err := PrepareConfig(context.Background(), input)
				if value == field.ceiling {
					if err != nil || result == nil {
						t.Fatal("inclusive native duration ceiling refused")
					}
				} else {
					requireConfigField(t, result, err, field.name)
				}
			}
		})
	}
}
