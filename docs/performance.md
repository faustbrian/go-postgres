# Performance

Benchmarks report allocations for configuration parsing, classification, pool
construction, acquisition wrapper overhead, transaction runner overhead,
observer dispatch, the OpenTelemetry adapter, real pooled acquire, and real
transaction round trips:

```sh
make benchmark BENCH_TIME=100ms
```

Results are environment-specific evidence, not service-level objectives. The
wrapper overhead benchmark uses a deterministic backend to isolate package
cost; integration benchmarks include PostgreSQL and container/network cost.

Optimize only after measuring the deployed workload. Connection establishment,
TLS, server execution, locks, rows, codecs, and exporter work normally dominate
these helpers. Keep telemetry labels bounded, avoid unnecessary parsing on hot
paths, reuse the pool, and do not trade cancellation or cleanup safety for a
microbenchmark improvement.
