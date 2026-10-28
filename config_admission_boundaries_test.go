package postgres

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
)

// Explicit ceilings are inclusive: the ceiling is admitted and one above it is
// refused. Every ceiling equals its default except the message-body budget.
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
		{"message", MaximumProtocolMessageBodyCeiling, func(l *ConfigLimits, n int) { l.MaximumProtocolMessageBodyBytes = n }},
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

// Only the message-body budget may exceed its default, and only on request.
func TestConfigAdmissionProtocolMessageBudgetDefaultAndRaise(t *testing.T) {
	input := admittedInput()
	native, err := PrepareConfig(context.Background(), input)
	if err != nil || native.ConnConfig.MaxProtocolMessageBodyLen != DefaultMaximumProtocolMessageBodyBytes {
		t.Fatal("zero message budget did not select the default")
	}
	input.Limits.MaximumProtocolMessageBodyBytes = DefaultMaximumProtocolMessageBodyBytes + 1
	native, err = PrepareConfig(context.Background(), input)
	if err != nil || native.ConnConfig.MaxProtocolMessageBodyLen != DefaultMaximumProtocolMessageBodyBytes+1 {
		t.Fatal("raised message budget was not applied to the native configuration")
	}
	input = admittedInput()
	input.Limits.MaximumDSNBytes = DefaultMaximumDSNBytes + 1
	result, err := PrepareConfig(context.Background(), input)
	requireConfigField(t, result, err, "limits")
}

// A raised budget bounds resolver and Configure native values by that budget,
// not by the default or the ceiling.
func TestConfigAdmissionRaisedProtocolMessageBudgetBoundsNativeValues(t *testing.T) {
	const budget = 16 << 20
	for _, test := range []struct {
		name  string
		value int
		field string
	}{
		{"between default and budget", DefaultMaximumProtocolMessageBodyBytes + 1, ""},
		{"at budget", budget, ""},
		{"over budget", budget + 1, "native_config"},
	} {
		t.Run("resolver "+test.name, func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumProtocolMessageBodyBytes = budget
			input.ResolveDSN = func(context.Context, string) (*PoolConfig, error) {
				native := admittedNative()
				native.ConnConfig.MaxProtocolMessageBodyLen = test.value
				return native, nil
			}
			result, err := PrepareConfig(context.Background(), input)
			if test.field != "" {
				requireConfigField(t, result, err, test.field)
			} else if err != nil || result.ConnConfig.MaxProtocolMessageBodyLen != test.value {
				t.Fatal("resolver message allowance within the raised budget refused")
			}
		})
		t.Run("configure "+test.name, func(t *testing.T) {
			input := admittedInput()
			input.Limits.MaximumProtocolMessageBodyBytes = budget
			input.Configure = func(_ context.Context, native *PoolConfig) error {
				native.ConnConfig.MaxProtocolMessageBodyLen = test.value
				return nil
			}
			result, err := PrepareConfig(context.Background(), input)
			if test.field != "" {
				requireConfigField(t, result, err, test.field)
			} else if err != nil || result.ConnConfig.MaxProtocolMessageBodyLen != test.value {
				t.Fatal("configured message allowance within the raised budget refused")
			}
		})
	}
}

// The ceiling must stay equal to pgx's own limit, which is unexported: a body
// one byte above the ceiling is the first one pgx's default frontend refuses.
func TestProtocolMessageBodyCeilingMatchesPgx(t *testing.T) {
	header := make([]byte, 5)
	header[0] = 'D'
	binary.BigEndian.PutUint32(header[1:], uint32(MaximumProtocolMessageBodyCeiling+1+4))
	frontend := pgproto3.NewFrontend(bytes.NewReader(header), io.Discard)
	_, err := frontend.Receive()
	var exceeded *pgproto3.ExceededMaxBodyLenErr
	if !errors.As(err, &exceeded) || exceeded.MaxExpectedBodyLen != MaximumProtocolMessageBodyCeiling {
		t.Fatalf("pgx native limit diverged from MaximumProtocolMessageBodyCeiling: %v", err)
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
