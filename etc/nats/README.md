# Testing NATS

Notes on testing embedded NATS features.

## Gateways

- Run 4 AnyCable RPC servers:

```sh
ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_NATS_SERVERS=nats://localhost:4242 \
ANYCABLE_RPC_HOST=127.0.0.1:50052 anyt --only-rpc

ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_NATS_SERVERS=nats://localhost:4243 \
ANYCABLE_RPC_HOST=127.0.0.1:50053 anyt --only-rpc

ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_NATS_SERVERS=nats://localhost:4244 \
ANYCABLE_RPC_HOST=127.0.0.1:50054 anyt --only-rpc

ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_NATS_SERVERS=nats://localhost:4245 \
ANYCABLE_RPC_HOST=127.0.0.1:50055 anyt --only-rpc
```

- Run 2 clusters with different names and connected to each other via gateways:

```sh
# Cluster 1, server 1
ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_EMBED_NATS=true \
ANYCABLE_RPC_HOST=127.0.0.1:50052 \
ANYCABLE_ENATS_ADDR=nats://localhost:4242 \
ANYCABLE_ENATS_CLUSTER=nats://localhost:4342 \
ANYCABLE_ENATS_GATEWAY=nats://localhost:4442 \
PORT=8082 make run

# Cluster 1, server 2
ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_EMBED_NATS=true \
ANYCABLE_RPC_HOST=127.0.0.1:50053 \
ANYCABLE_ENATS_ADDR=nats://localhost:4243 \
ANYCABLE_ENATS_CLUSTER=nats://localhost:4343 \
ANYCABLE_ENATS_CLUSTER_ROUTES=nats://localhost:4342 \
ANYCABLE_ENATS_GATEWAY=nats://localhost:4443 \
PORT=8083 make run

# Cluster 2, server 1
ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_EMBED_NATS=true \
ANYCABLE_RPC_HOST=127.0.0.1:50054 \
ANYCABLE_ENATS_CLUSTER_NAME=anycable-cluster-2 \
ANYCABLE_ENATS_ADDR=nats://localhost:4244 \
ANYCABLE_ENATS_CLUSTER=nats://localhost:4344 \
ANYCABLE_ENATS_GATEWAY=nats://localhost:4444 \
ANYCABLE_ENATS_GATEWAYS=anycable-cluster:nats://localhost:4442 \
PORT=8084 make run

# Cluster 2, server 2
ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_EMBED_NATS=true \
ANYCABLE_RPC_HOST=127.0.0.1:50055 \
ANYCABLE_ENATS_CLUSTER_NAME=anycable-cluster-2 \
ANYCABLE_ENATS_ADDR=nats://localhost:4245 \
ANYCABLE_ENATS_CLUSTER=nats://localhost:4345 \
ANYCABLE_ENATS_CLUSTER_ROUTES=nats://localhost:4344 \
ANYCABLE_ENATS_GATEWAY=nats://localhost:4445 \
ANYCABLE_ENATS_GATEWAYS=anycable-cluster:nats://localhost:4442 \
PORT=8085 make run
```

- Use `acli` to connect to 4 clients and try to perform the "broadcast" action—all server should receive it:

```sh
# client 1
acli -u localhost:8082/cable -c BenchmarkChannel

# client 2
acli -u localhost:8083/cable -c BenchmarkChannel

# client 3
acli -u localhost:8084/cable -c BenchmarkChannel

# client 4
$ acli -u localhost:8085/cable -c BenchmarkChannel

\p+ broadcast
Enter key (or press ENTER to finish): test
Enter value: gossip
Enter key (or press ENTER to finish):

{"test":"gossip","action":"broadcastResult"}
{"action":"broadcast","test":"gossip"}
```
