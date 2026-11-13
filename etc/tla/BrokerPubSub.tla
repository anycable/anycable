---------------------------- MODULE BrokerPubSub ----------------------------
(*
  Broker stream tracking and the pub/sub subscriber for a single stream,
  with concurrent sessions subscribing to and unsubscribing from it.
  Reflects the current code.

  Processes:
    - clients: sessions calling broker.Subscribe / broker.Unsubscribe for the stream
      (from handleCommandReply, TryRestoreSession, Node.Disconnect), in turn, any
      number of times. Different sessions don't share s.rmu, so calls are concurrent;
      the subscriber is called under tracker.mu.
    - ps (Adapter = "redis"): RedisSubscriber.runPubSub / maybeReconnect; the
      pub/sub connection drops at most MaxDrops times.
*)
EXTENDS Integers, FiniteSets, TLC

CONSTANTS
  Clients,   \* sessions
  Adapter,   \* "redis" (RedisSubscriber) or "nats" (NATSSubscriber)
  MaxDrops   \* Redis pub/sub connection drops

Procs == Clients \cup {"ps"}

VARIABLES
  tracker,     \* StreamsTracker.store[stream] (0 = absent)
  trackerMu,   \* StreamsTracker.mu holder ("none" = free)
  psMap,       \* Subscriber.subscriptions has the stream (subMu)
  psClient,    \* RedisSubscriber.psClient: connection id (0 = nil) (psMu)
  redisConn,   \* live Redis pub/sub connection id (0 = none)
  redisSubs,   \* the live Redis connection is subscribed to the stream
  natsSubs,    \* live NATS subscriptions for the stream
  drops,       \* Redis connection drops so far
  cliConn,     \* clients: psClient read by Subscribe/Unsubscribe
  pc

brokerVars == <<tracker, trackerMu>>
psVars     == <<psMap, psClient, redisConn, redisSubs, natsSubs, drops>>
localVars  == <<cliConn>>
vars       == <<brokerVars, psVars, localVars, pc>>

Init ==
  /\ tracker = 0
  /\ trackerMu = "none"
  /\ psMap = FALSE
  /\ psClient = 1
  /\ redisConn = 1
  /\ redisSubs = FALSE
  /\ natsSubs = 0
  /\ drops = 0
  /\ cliConn = [c \in Clients |-> 0]
  /\ pc = [p \in Procs |-> IF p = "ps" THEN "Live" ELSE "Idle"]

Goto(p, l) == pc' = [pc EXCEPT ![p] = l]

SubscriberSub == IF Adapter = "redis" THEN "RedisSubMap" ELSE "NatsSubChk"
SubscriberUnsub == IF Adapter = "redis" THEN "RedisUnsubMap" ELSE "NatsUnsubLock"

-----------------------------------------------------------------------------
(* broker *)

\* broker.Subscribe: tracker.Add(stream, onNew) — tracker.mu.Lock(); store[stream]++; onNew() (broadcaster.Subscribe) if new
BrokerSubAdd(c) ==
  /\ pc[c] = "Idle"
  /\ trackerMu = "none"
  /\ tracker' = tracker + 1
  /\ trackerMu' = IF tracker = 0 THEN c ELSE "none"
  /\ Goto(c, IF tracker = 0 THEN SubscriberSub ELSE "Held")
  /\ UNCHANGED <<psVars, localVars>>

\* tracker.Add: tracker.mu.Unlock() after onNew
BrokerSubUnlock(c) ==
  /\ pc[c] = "BrokerSubUnlock"
  /\ trackerMu' = "none"
  /\ Goto(c, "Held")
  /\ UNCHANGED <<tracker, psVars, localVars>>

\* broker.Unsubscribe: tracker.Remove(stream, onLast) — tracker.mu.Lock(); store[stream]--; onLast() (broadcaster.Unsubscribe) if last
BrokerUnsubRemove(c) ==
  /\ pc[c] = "Held"
  /\ trackerMu = "none"
  /\ tracker' = tracker - 1
  /\ trackerMu' = IF tracker = 1 THEN c ELSE "none"
  /\ Goto(c, IF tracker = 1 THEN SubscriberUnsub ELSE "Idle")
  /\ UNCHANGED <<psVars, localVars>>

\* tracker.Remove: tracker.mu.Unlock() after onLast
BrokerUnsubUnlock(c) ==
  /\ pc[c] = "BrokerUnsubUnlock"
  /\ trackerMu' = "none"
  /\ Goto(c, "Idle")
  /\ UNCHANGED <<tracker, psVars, localVars>>

-----------------------------------------------------------------------------
(* RedisSubscriber *)

\* redis.go: Subscribe — subscriptions[stream] = entry (subMu)
RedisSubMap(c) ==
  /\ pc[c] = "RedisSubMap"
  /\ psMap' = TRUE
  /\ Goto(c, "RedisSubClient")
  /\ UNCHANGED <<brokerVars, psClient, redisConn, redisSubs, natsSubs, drops, localVars>>

\* redis.go: Subscribe — client := s.pubsubClient() (psMu); return if nil
RedisSubClient(c) ==
  /\ pc[c] = "RedisSubClient"
  /\ cliConn' = [cliConn EXCEPT ![c] = psClient]
  /\ Goto(c, IF psClient = 0 THEN "BrokerSubUnlock" ELSE "RedisSubDo")
  /\ UNCHANGED <<brokerVars, psVars>>

\* redis.go: Subscribe — client.Do(SUBSCRIBE) (no effect on a dead connection)
RedisSubDo(c) ==
  /\ pc[c] = "RedisSubDo"
  /\ redisSubs' = IF cliConn[c] = redisConn THEN TRUE ELSE redisSubs
  /\ Goto(c, "BrokerSubUnlock")
  /\ UNCHANGED <<brokerVars, psMap, psClient, redisConn, natsSubs, drops, localVars>>

\* redis.go: Unsubscribe — return if not in subscriptions; delete(subscriptions, stream) (subMu)
RedisUnsubMap(c) ==
  /\ pc[c] = "RedisUnsubMap"
  /\ psMap' = FALSE
  /\ Goto(c, IF psMap THEN "RedisUnsubClient" ELSE "BrokerUnsubUnlock")
  /\ UNCHANGED <<brokerVars, psClient, redisConn, redisSubs, natsSubs, drops, localVars>>

\* redis.go: Unsubscribe — client := s.pubsubClient() (psMu); return if nil
RedisUnsubClient(c) ==
  /\ pc[c] = "RedisUnsubClient"
  /\ cliConn' = [cliConn EXCEPT ![c] = psClient]
  /\ Goto(c, IF psClient = 0 THEN "BrokerUnsubUnlock" ELSE "RedisUnsubDo")
  /\ UNCHANGED <<brokerVars, psVars>>

\* redis.go: Unsubscribe — client.Do(UNSUBSCRIBE) (no effect on a dead connection)
RedisUnsubDo(c) ==
  /\ pc[c] = "RedisUnsubDo"
  /\ redisSubs' = IF cliConn[c] = redisConn THEN FALSE ELSE redisSubs
  /\ Goto(c, "BrokerUnsubUnlock")
  /\ UNCHANGED <<brokerVars, psMap, psClient, redisConn, natsSubs, drops, localVars>>

\* runPubSub: <-wait fires (connection lost)
ReconnDrop ==
  /\ Adapter = "redis"
  /\ pc["ps"] = "Live"
  /\ drops < MaxDrops
  /\ drops' = drops + 1
  /\ redisConn' = 0
  /\ redisSubs' = FALSE
  /\ Goto("ps", "ReconnNil")
  /\ UNCHANGED <<brokerVars, psMap, psClient, natsSubs, localVars>>

\* maybeReconnect: s.psClient = nil (psMu)
ReconnNil ==
  /\ pc["ps"] = "ReconnNil"
  /\ psClient' = 0
  /\ Goto("ps", "ReconnDial")
  /\ UNCHANGED <<brokerVars, psMap, redisConn, redisSubs, natsSubs, drops, localVars>>

\* runPubSub: s.client.Dedicate(); s.psClient = client (psMu)
ReconnDial ==
  /\ pc["ps"] = "ReconnDial"
  /\ redisConn' = drops + 1
  /\ psClient' = drops + 1
  /\ Goto("ps", "ResubBatch")
  /\ UNCHANGED <<brokerVars, psMap, redisSubs, natsSubs, drops, localVars>>

\* resubscribe: channels := maps.Keys(s.subscriptions); batchSubscribe(client, channels) (subMu.RLock)
ResubBatch ==
  /\ pc["ps"] = "ResubBatch"
  /\ redisSubs' = IF psMap /\ psClient = redisConn THEN TRUE ELSE redisSubs
  /\ Goto("ps", "Live")
  /\ UNCHANGED <<brokerVars, psMap, psClient, redisConn, natsSubs, drops, localVars>>

-----------------------------------------------------------------------------
(* NATSSubscriber *)

\* nats.go: Subscribe — return if in subscriptions (subMu.RLock)
NatsSubChk(c) ==
  /\ pc[c] = "NatsSubChk"
  /\ Goto(c, IF psMap THEN "BrokerSubUnlock" ELSE "NatsSubLock")
  /\ UNCHANGED <<brokerVars, psVars, localVars>>

\* nats.go: Subscribe — conn.Subscribe; subscriptions[stream] = sub (subMu)
NatsSubLock(c) ==
  /\ pc[c] = "NatsSubLock"
  /\ natsSubs' = natsSubs + 1
  /\ psMap' = TRUE
  /\ Goto(c, "BrokerSubUnlock")
  /\ UNCHANGED <<brokerVars, psClient, redisConn, redisSubs, drops, localVars>>

\* nats.go: Unsubscribe — if in subscriptions: delete, sub.Unsubscribe() (subMu)
NatsUnsubLock(c) ==
  /\ pc[c] = "NatsUnsubLock"
  /\ psMap' = FALSE
  /\ natsSubs' = IF psMap THEN natsSubs - 1 ELSE natsSubs
  /\ Goto(c, "BrokerUnsubUnlock")
  /\ UNCHANGED <<brokerVars, psClient, redisConn, redisSubs, drops, localVars>>

-----------------------------------------------------------------------------

Client(c) ==
  \/ BrokerSubAdd(c) \/ BrokerSubUnlock(c) \/ BrokerUnsubRemove(c) \/ BrokerUnsubUnlock(c)
  \/ RedisSubMap(c) \/ RedisSubClient(c) \/ RedisSubDo(c)
  \/ RedisUnsubMap(c) \/ RedisUnsubClient(c) \/ RedisUnsubDo(c)
  \/ NatsSubChk(c) \/ NatsSubLock(c) \/ NatsUnsubLock(c)

PubSub == ReconnDrop \/ ReconnNil \/ ReconnDial \/ ResubBatch

Next == (\E c \in Clients : Client(c)) \/ PubSub

Spec == Init /\ [][Next]_vars

Quiescent ==
  /\ \A c \in Clients : pc[c] \in {"Idle", "Held"}
  /\ pc["ps"] = "Live"

Holders ==
  {c \in Clients : pc[c] \in {"Held", "RedisSubMap", "RedisSubClient", "RedisSubDo", "NatsSubChk", "NatsSubLock", "BrokerSubUnlock"}}

-----------------------------------------------------------------------------

TypeOK ==
  /\ tracker \in 0..Cardinality(Clients)
  /\ trackerMu \in Clients \cup {"none"}
  /\ psMap \in BOOLEAN
  /\ redisSubs \in BOOLEAN
  /\ natsSubs \in Nat

\* The tracker counts the sessions holding the stream
TrackerExact == tracker = Cardinality(Holders)

\* The subscriber tracks the stream iff some session holds it
SubscriberInSync == Quiescent => (psMap <=> tracker > 0)

\* Messages for the stream arrive iff some session holds it (exactly once)
WireInSync ==
  Quiescent =>
    IF Adapter = "redis"
      THEN redisSubs <=> tracker > 0
      ELSE natsSubs = IF tracker > 0 THEN 1 ELSE 0

=============================================================================
