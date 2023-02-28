# Embedded NATS

AnyCable supports running a NATS server as a part of the `anycable-go` WebSocket server. Thus, you don't need any _external_ pub/sub services to build AnyCable clusters (i.e., having multiple WebSocket nodes).

There are multiple ways to use this functionality:

- [Single-server configuration](#single-server-configuration)
- [Cluster configuration](#cluster-configuration)

## Single-server configuration

The easiest way to start using embedded NATS in AnyCable is to run a single `anycable-go` instance with _eNATS_ (this is how we call "Embedded NATS") enabled and connecting all other instances to it. This is how you can do that locally:

```sh
# first instance with NATS embedded
$ anycable-go --broadcast_adapter=nats --embed_nats --enats_addr=nats://0.0.0.0:4242

INFO 2023-02-28T00:06:45.618Z context=main Starting AnyCable 1.3.0
INFO 2023-02-28T00:06:45.649Z context=main Embedded NATS server started: nats://127.0.0.1:4242
```

Now you can run another WebSocket server connected to the first one:

```sh
anycable-go --port 8081 --broadcast_adapter=nats --nats_servers=nats://0.0.0.0:4242
```

RPC servers can also connect to the first AnyCable-Go server:

```sh
bundle exec anycable --broadcast_adapter=nats --nats_servers=nats://0.0.0.0:4242
```

This setup is similar to running a single NATS server independently.

## Cluster configuration

Alternatively, you can form a cluster from embedded NATS instances. For that, you should start each `anycable-go` instance with a NATS cluster address and connect them together via the routes table:

```sh
# first instance
$ anycable-go --broadcast_adapter=nats --embed_nats --enats_addr=nats://0.0.0.0:4242 --enats_cluster=nats://0.0.0.0:4243

INFO 2023-02-28T00:06:45.618Z context=main Starting AnyCable 1.3.0
INFO 2023-02-28T00:06:45.649Z context=main Embedded NATS server started: nats://127.0.0.1:4242 (cluster: nats://0.0.0.0:4243, cluster_name: anycable-cluster)

# other instances
$ anycable-go --port 8081 --broadcast_adapter=nats --embed_nats --enats_addr=nats://0.0.0.0:4342 --enats_cluster=nats://0.0.0.0:4343 --enats_cluster_routes=nats://0.0.0.0:4243

INFO 2023-02-28T00:06:45.618Z context=main Starting AnyCable 1.3.0
INFO 2023-02-28T00:06:45.649Z context=main Embedded NATS server started: nats://127.0.0.1:4342 (cluster: nats://0.0.0.0:4343, cluster_name: anycable-cluster, routes: nats://0.0.0.0:4243)
```

See more information in the [NATS documentation](https://docs.nats.io/running-a-nats-service/configuration/clustering).

### Super-cluster

You can also setup a super-cluster by configuring gateways:

```sh
# first cluster
$ anycable-go --broadcast_adapter=nats --embed_nats \
--enats_addr=nats://0.0.0.0:4242 --enats_cluster=nats://0.0.0.0:4243 \
--enats_gateway=nats://0.0.0.0:7222

# second cluster
$ anycable-go --port 8081 --broadcast_adapter=nats --embed_nats \
--enats_addr=nats://0.0.0.0:4342 --enats_cluster=nats://0.0.0.0:4343 \
--enats_gateway=nats://0.0.0.0:7322 \
--enats_gateways=anycable-cluster:nats://0.0.0.0:7222
```

**NOTE**: The value of the `--enats_gateways` parameter must be have a form `<name>:<addr-1>,<addr-2>;<name-2>:<addr-3>,<addr-4>`.

**IMPORTANT**: All servers in the cluster must have the same gateway configuration.

See more information in the [NATS documentation](https://docs.nats.io/running-a-nats-service/configuration/clustering).
