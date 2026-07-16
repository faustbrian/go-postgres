package postgres

import (
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func FuzzParseConfig(f *testing.F) {
	f.Add("postgres://app:secret@localhost/app?sslmode=disable")
	f.Add("host=/var/run/postgresql user=app dbname=app")
	f.Add("postgres://app:p%40ss@[::1]:5432/app?sslmode=require")
	f.Add("not a valid DSN password=fuzz-secret")

	f.Fuzz(func(t *testing.T, dsn string) {
		const marker = "fuzz-secret"
		_, err := ParseConfig(Config{DSN: dsn})
		if err != nil && strings.Contains(err.Error(), marker) {
			t.Fatalf("ParseConfig() leaked secret marker: %v", err)
		}
	})
}

func FuzzOptions(f *testing.F) {
	f.Add(uint8(0), uint8(0), int64(0), int32(10), int32(0))
	f.Add(uint8(255), uint8(255), int64(-1), int32(-1), int32(-1))

	f.Fuzz(func(
		t *testing.T,
		startupPolicy uint8,
		tlsMode uint8,
		cleanupNanoseconds int64,
		maxConns int32,
		minConns int32,
	) {
		_, _ = ParseConfig(Config{
			DSN:           "postgres://localhost/app?sslmode=disable",
			StartupPolicy: StartupPolicy(startupPolicy),
			TLS: TLSConfig{
				Mode:   TLSMode(tlsMode),
				Config: &tls.Config{MinVersion: tls.VersionTLS12},
			},
			MaxConns: maxConns,
			MinConns: minConns,
		})
		_ = TransactionOptions{CleanupTimeout: time.Duration(cleanupNanoseconds)}
		_ = SavepointOptions{CleanupTimeout: time.Duration(cleanupNanoseconds)}
	})
}

func FuzzRedaction(f *testing.F) {
	f.Add("secret", "host name")
	f.Add("p@ss word", "[::1")
	f.Add("'\\\"", "%invalid")

	f.Fuzz(func(t *testing.T, password, host string) {
		secret := "fuzz-secret-" + hex.EncodeToString([]byte(password))
		dsn := fmt.Sprintf(
			"postgres://app:%s@%s/app?sslmode=verify-full&invalid=1",
			url.PathEscape(secret), host,
		)
		_, err := ParseConfig(Config{DSN: dsn})
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatalf("ParseConfig() leaked password marker")
		}
	})
}

func FuzzSQLStateClassification(f *testing.F) {
	for _, code := range []string{"23505", "23503", "40001", "40P01", "57014", "08006", "ZZZZZ"} {
		f.Add(code)
	}

	f.Fuzz(func(t *testing.T, code string) {
		info := Classify(&pgconn.PgError{Code: code})
		if info.SQLState != code {
			t.Fatalf("Classify(%q).SQLState = %q", code, info.SQLState)
		}
		if state, ok := SQLState(info.Cause); !ok || state != code {
			t.Fatalf("SQLState(%q) = %q, %v", code, state, ok)
		}
	})
}

func FuzzConfigurationBounds(f *testing.F) {
	f.Add(int32(10), int32(0), int32(0), int64(5))
	f.Add(int32(-1), int32(0), int32(0), int64(-1))

	f.Fuzz(func(t *testing.T, maxConns, minConns, minIdleConns int32, timeoutMilliseconds int64) {
		_, _ = ParseConfig(Config{
			DSN:            "postgres://localhost/app?sslmode=disable",
			MaxConns:       maxConns,
			MinConns:       minConns,
			MinIdleConns:   minIdleConns,
			ConnectTimeout: durationFromFuzzMilliseconds(timeoutMilliseconds),
		})
	})
}

func durationFromFuzzMilliseconds(value int64) time.Duration {
	const maxMilliseconds = int64((24 * time.Hour) / time.Millisecond)
	if value > maxMilliseconds {
		value = maxMilliseconds
	}
	if value < -maxMilliseconds {
		value = -maxMilliseconds
	}

	return time.Duration(value) * time.Millisecond
}
