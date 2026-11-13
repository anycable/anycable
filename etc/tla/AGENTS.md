# TLA+ specs: conventions

## Files

- `<Spec>.tla` reflects the current code; don't model fixes there.
- `<Spec>Proposal.tla` is a copy with candidate fixes, each marked with a `PROPOSAL:` note.
- Check with the bundled `manyan` (see README.md); don't install Java/TLC.
- Don't keep temporary configs or scripts here.

## Modeling

- One action = one atomic step = one lock-held region (or one atomic op) in the Go code. Unlocked gaps between regions are where other processes interleave. The comment above each action points to that code (`file.go: Func — detail`).
- RPC = request step + response step with a nondeterministic outcome.
- Cleanliness invariants are checked at quiescence (all processes done).
- Spec comments describe the Go code being modeled, nothing else.

## Naming

Actions (and their pc labels, which are the same) are named `<Scope><Step>` after the Go code:

| Scope | Go code |
|---|---|
| `Auth*`, `Sub*`, `Unsub*` (reader) | `Node.Authenticate` / `Subscribe` / `Unsubscribe` |
| `ServeLoop`, `SubCmd` | `Session.Serve` loop (reading the next command) |
| `SubReply*`, `ReplyStreamsLoop`, `Stop*` | `handleCommandReply` and the `s.rmu` region around it (from `Subscribe`: `reply.Streams`; from `Unsubscribe`: `StopAllStreams`) |
| `State*` | `SubscriptionState` |
| `Broker*` | broker |
| `Hub*` | hub, gate |
| `HubRemoveSes*` | `Hub.RemoveSession` |
| `CloserStart`, `SessionClose`, `DisconnectConnected` | `session.go` close paths |
| `DisconnectLock`, `DisconnectUnlock` | `Node.Disconnect` (`s.rmu` region) |
| `UnsubSnapStreams`, `UnsubBrokerUnsubLoop` (closers) | `Node.unsubscribeSessionStreams` |
| `Undo*` | rollback steps (proposals) |
| `BrokerSubAdd`, `BrokerUnsubRemove` | broker `Subscribe`/`Unsubscribe` (`StreamsTracker`) |
| `Redis*`, `Nats*` | `RedisSubscriber` / `NATSSubscriber` `Subscribe`/`Unsubscribe` |
| `Reconn*`, `Resub*` | `RedisSubscriber.runPubSub` / `maybeReconnect` / `resubscribe` |

Step suffixes:

- `*RPC`: the RPC call; `*RPCChk`: the `IsClosed()`/result check after it.
- `*Chk`: a check-only step (e.g. an RLock check).
- `*Loop`: a step that iterates over a set.
- `*Lock`/`*Unlock`: acquiring/releasing a mutex held across several steps.

Variables are prefixed by their owner:

- `subs*`: `Session.subscriptions` (`subsChan`, `subsStreams`, `subsSnapshot`, `subsFrozen`).
- `rmu`: `Session.rmu` holder (mutexes held across steps are modeled as a holder variable named after the Go field).
- `hub*`: Hub and Gate (`hubSessions`, `hubStreams`, `hubGate`).
- `tracker`, `pubsub`: broker `StreamsTracker`, pub/sub subscriber.
- `ps*`, `redis*`, `nats*`: `Subscriber` state (`psMap`, `psClient`), Redis/NATS server-side state (`redisConn`, `redisSubs`, `natsSubs`).
- `todo*`, `cur*`, `rpc*`, `cli*`, `resub*`: process locals (`todoSubStreams`, `todoUnsubStreams`, `curStream`, `rpcResult`, `cliConn`, `resubSnap`).
- Ghost variables (not in the code) are documented as `ghost:` (e.g. `refs`).
- Variable groups for `UNCHANGED` are `<owner>Vars` (`sessVars`, `hubVars`, `brokerVars`, `readerVars`).
