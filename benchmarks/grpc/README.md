# gRPC server benchmarking

## How to run

First, run AnyCable RPC server:

```sh
$ ruby server.rb

Starting AnyT v1.2.0 (pid: 46213)

```

The `server.rb` script is self-contained and use [AnyT](https://github.com/anycable/anyt) to setup a Rails application and launch an RPC server. The application provides a [BenchmarkChannel](https://github.com/anycable/anyt/blob/ee8c622ff1b3c0431435a65e1047632c078208ba/lib/anyt/dummy/application.rb#L46).

Then, run [k6](https://k6.io):

```sh
# Run a single gRPC connection for 10s
k6 run echo.js --duration 10s

# Run 30 connections for 30s
k6 run echo.js --duration 30s --vus 30
```

## Results

### 2022-02-08

Setup:

- Apple M1
- Ruby 3.1
- `grpc` 1.50.0
- `grpc_kit` 0.5.1

Running `k6 run echo.js --duration=10s  --vus=30`:

- gRPC

```
running (10.6s), 00/30 VUs, 302 complete and 0 interrupted iterations
default ✓ [======================================] 30 VUs  10s

     ✗ status is OK
      ↳  99% — ✓ 30190 / ✗ 10

     checks...............: 99.96% ✓ 30190     ✗ 10
     data_received........: 5.9 MB 554 kB/s
     data_sent............: 5.9 MB 562 kB/s
     grpc_connect.........: avg=3.69ms  min=0s       med=4ms    max=14ms    p(90)=6ms     p(95)=7ms
     grpc_req_duration....: avg=10.23ms min=342.16µs med=9.77ms max=40.59ms p(90)=15.07ms p(95)=16.96ms
     iteration_duration...: avg=1.03s   min=584.2ms  med=1.04s  max=1.19s   p(90)=1.1s    p(95)=1.13s
     iterations...........: 302    28.568183/s
     vus..................: 30     min=30      max=30
     vus_max..............: 30     min=30      max=30
```

**NOTE:** A few failures due to "No free threads in thread pool".

- grpc_kit

```
running (11.6s), 00/30 VUs, 218 complete and 0 interrupted iterations
default ✓ [======================================] 30 VUs  10s

     ✓ status is OK

     checks...............: 100.00% ✓ 21800     ✗ 0
     data_received........: 3.5 MB  305 kB/s
     data_sent............: 4.3 MB  368 kB/s
     grpc_connect.........: avg=1.43s    min=2ms      med=1.51s   max=1.67s p(90)=1.61s    p(95)=1.62s
     grpc_req_duration....: avg=492.22µs min=366.54µs med=451.5µs max=5.4ms p(90)=615.55µs p(95)=660.91µs
     iteration_duration...: avg=1.48s    min=57.8ms   med=1.56s   max=1.73s p(90)=1.66s    p(95)=1.68s
     iterations...........: 218     18.770996/s
     vus..................: 11      min=11      max=30
     vus_max..............: 30      min=30      max=30
```

Conclusions:

- `grpc_kit` connection times are much slower (~1.5s > 7ms).
- `grpc_kit` performs much better for simple requests (p95: 1ms < 16ms).
- `grpc`'s thread pool doesn't work correctly (release workers asynchronously) resulting in "No free threads in thread pool" errors.
