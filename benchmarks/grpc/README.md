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

### 2022-02-03


Running `k6 run echo.js --duration=10s  --vus=30`:

- gRPC

```
running (10.1s), 00/30 VUs, 300 complete and 0 interrupted iterations
default ✓ [======================================] 30 VUs  10s

     ✓ status is OK

     checks...............: 100.00% ✓ 300       ✗ 0   
     data_received........: 101 kB  10 kB/s
     data_sent............: 97 kB   9.6 kB/s
     grpc_req_duration....: avg=6.74ms min=1.18ms med=6.49ms max=18.95ms p(90)=10.88ms p(95)=12.11ms
     iteration_duration...: avg=1s     min=1s     med=1s     max=1.02s   p(90)=1.01s   p(95)=1.01s  
     iterations...........: 300     29.695843/s
     vus..................: 30      min=30      max=30
     vus_max..............: 30      min=30      max=30
```

- grpc_kit

```
running (10.1s), 00/30 VUs, 300 complete and 0 interrupted iterations
default ✓ [======================================] 30 VUs  10s

     ✓ status is OK

     checks...............: 100.00% ✓ 300       ✗ 0
     data_received........: 58 kB   5.7 kB/s
     data_sent............: 92 kB   9.1 kB/s
     grpc_req_duration....: avg=967.66µs min=446.91µs med=864.79µs max=5.82ms p(90)=1.37ms p(95)=1.58ms
     iteration_duration...: avg=1s       min=1s       med=1s       max=1.02s  p(90)=1.01s  p(95)=1.01s 
     iterations...........: 300     29.687503/s
     vus..................: 30      min=30      max=30
     vus_max..............: 30      min=30      max=30
```

`grpc_kit` performs much better for simple requests (p85: 1.5ms < 12ms).
