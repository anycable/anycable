# Broker deep dive

Broker is a component of AnyCable-Go responsible for keeping streams and sessions information in a cache-like storage. It drives the [Reliable Streams](./reliable_streams.md) feature.

Broker implements features that can be characterized as _hot cache utilities_:

- Handling incoming broadcast messages and storing them in a cache—that could help clients to receive missing broadcasts (triggered while the client was offline, for example).
- Persisting client states—to make it possible to restore on re-connection (by providing a _session id_ of the previous connection).

## Client-server communication

Below you can see the diagram demonstrating how clients can you the broker-backed features to keep up with the stream messages and restore their state:

```mermaid
sequenceDiagram
    participant Client
    participant Server
    participant RPC
    participant Publisher
    Publisher--)Server: '{"stream":"chat_42","data":{"text":"Hi"}}'
    Client->>Server: CONNECT /cable
    activate Client
    Server->>RPC: Connect
    RPC->>Server: SUCCESS
    Server->>Client: '{"type":"welcome","sid":"a431"}'
    Client->>Server: '{"command":"subscribe","identifier":"ChatChannel/42","history":{"since":163213232}}'
    Server->>RPC: Subscribe
    RPC->>Server: SUCCESS
    Server->>Client: '{"type":"confirm_subscription"}}'
    Server->>Client: '{"message":{"text":"Hi"},"stream_id":"chat_42",offset: 42, epoch: "y2023"}'
    Server->>Client: '{"type":"confirm_history"}'
    Publisher--)Server: '{"stream":"chat_42","data":{"text":"What's up?"}}'
    Server->>Client: '{"message":{"text":"What's up?"},"stream_id":"chat_42",offset: 43, epoch: "y2023"}'
    Client-x Client: DISCONNECT
    deactivate Client
    Server--)RPC: Disconnect
    Publisher--)Server: '{"stream":"chat_42","data":{"text":"Where are you?"}}'
    Client->>Server: CONNECT /cable?sid=a431
    activate Client
    Note over Server,RPC: No RPC calls made here
    Server->>Client: '{"type":"welcome", "sid":"h542", "restored":true,"restored_ids":["ChatChannel/42"]}'
    Note over Client,Server: No need to re-subscribe, we only request history
    Client->>Server: '{"type":"history","identifier":"ChatChannel/42","history":{"streams": {"chat_42": {"offset":43,"epoch":"y2023"}}}}'
    Server->>Client: '{"message":{"text":"Where are you?"},"stream_id":"chat_42",offset: 44, epoch: "y2023"}'
    Server->>Client: '{"type":"confirm_history"}'
    deactivate Client
```

To support these features, an [extended Action Cable protocol](/misc/action_cable_protocol.md#action-cable-extended-protocol) is used for communication.

You can use [AnyCable JS client](https://github.com/anycable/anycable-client) library at the client-side to use the extended protocol.

## Broadcasting messages

Broker is responsible for **registering broadcast messages**. Each message MUST be registered once; thus, we MUST you a broadcasting method which publishes messages to a single node in a cluster (see [Broadcast adapters](../ruby/broadcast_adapters.md)). Currently, `http` and `redisx` adapters are supported.

**NOTE:** When legacy adapters are used, enabling a broker has no effect.

To re-transmit registered messages within a cluster, we need a pub/sub component. See [Pub/Sub](./pubsub.md) for more information.

The overall broadcasting message flow looks as follows:

```mermaid
graph LR
  Publisher[Publisher]

  subgraph node2[Node 2]
   PubSub2[Pub/Sub 2]
   ClientC[Client C]
   ClientD[Client D]
  end

  subgraph node1[Node 1]
   Broadcaster[Broadcaster]
   Broker[Broker]
   BrokerBackend[Broker Backend]
   PubSub[Pub/Sub]
   ClientA[Client A]
   ClientB[Client B]
  end

  class node1 lightbg
  class node2 lightbg
  classDef lightbg fill:#ffe,stroke:#333,stroke-width:2px

  Publisher -.->|Message| Broadcaster
  Broadcaster -->|Message| Broker
  Broker -->|Cache Message| BrokerBackend
  BrokerBackend --> Broker
  Broker -->|Registered Message| PubSub
  PubSub -->|Registered Message| ClientA
  PubSub -->|Registered Message| ClientB

  PubSub -.-> PubSub2

  PubSub2 -->|Registered Message| ClientC
  PubSub2 -->|Registered Message| ClientD
```
