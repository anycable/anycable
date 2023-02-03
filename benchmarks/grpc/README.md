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
