# DNS load balancing playground

To play with the gRPC DNS load balancing feature, you need to run multiple RPC servers on different network interfaces and a DNS server.

## Running multiple RPC servers

On MacOS, you can add aliases for the loopback interface as follows:

```sh
sudo ifconfig lo0 alias 127.0.0.2
sudo ifconfig lo0 alias 127.0.0.3
sudo ifconfig lo0 alias 127.0.0.4
```

Now you can run an RPC server bound to one of the aliases:

```sh
ANYCABLE_RPC_HOST=127.0.0.2:50051 bundle exec anycable
ANYCABLE_RPC_HOST=127.0.0.3:50051 bundle exec anycable

# or
ANYCABLE_RPC_HOST=127.0.0.2:50051 anyt --only-rpc
ANYCABLE_RPC_HOST=127.0.0.3:50051 anyt --only-rpc


# the same for other addresses
```

## Running a DNS server

We use [async-dns](https://github.com/socketry/async-dns) to implement a simple DNS server resolving `anycable-rpc.local` to currently running RPC server' IPs:

```sh
$ ruby server.rb

  0.0s     info: Starting Async::DNS server (v1.3.0)... [ec=0x5dc] [pid=21146] [2023-02-21 10:08:53 -0500]
  0.0s     info: <> Listening for datagrams on #<Addrinfo: 127.0.0.1:2346 UDP> [ec=0x5f0] [pid=21146] [2023-02-21 10:08:53 -0500]
  0.0s     info: <> Listening for connections on #<Addrinfo: 127.0.0.1:2346 TCP> [ec=0x604] [pid=21146] [2023-02-21 10:08:53 -0500]
```

## Running anycable-go

Now you need to run anycable-go with the following RPC host configuration:

```sh
ANYCABLE_RPC_HOST=dns://127.0.0.1:2346/anycable-rpc.local:50051 anycable-go

# or
ANYCABLE_RPC_HOST=dns://127.0.0.1:2346/anycable-rpc.local:50051 make run
```

You should see the logs of connecting to multiple RPC servers:

```sh
INFO 2023-02-21T15:14:04.472Z context=main Starting AnyCable 1.2.3-aec3660
# ...
DEBUG 2023-02-21T15:14:04.682Z context=grpc connected to 127.0.0.2:50051
DEBUG 2023-02-21T15:14:04.682Z context=grpc connected to 127.0.0.1:50051
# ...
```
