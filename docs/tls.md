# TLS

PostgreSQL TLS has two separate goals: encrypting traffic and authenticating
the server. Production deployments normally need both.

`TLSFromDSN` preserves pgx DSN behavior. Prefer `sslmode=verify-full` with an
explicit root certificate and a hostname matching the certificate. Be cautious
with pgx or libpq modes that permit plaintext fallback.

`TLSRequire` applies a cloned `*tls.Config` to the primary and every fallback
host, eliminating plaintext fallback. The name means TLS transport is required;
the supplied configuration still controls authentication. Keep
`InsecureSkipVerify=false`, populate `RootCAs`, use TLS 1.2 or newer, and set a
valid `ServerName` when hostname inference is not appropriate.

`TLSDisable` removes TLS from the primary and every fallback and is intended
for local Testcontainers or a separately secured Unix/socket or sidecar path.
Document the compensating control before using it outside tests.

Certificate files are loaded by the application so secret delivery, rotation,
permissions, and reload policy remain explicit. Never log a DSN, private key,
client certificate secret, or full `tls.Config`.
