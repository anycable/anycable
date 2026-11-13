---------------------------- MODULE BrokerPubSub ----------------------------
(*
  Broker stream tracking and the pub/sub subscriber for a single stream,
  with concurrent sessions subscribing to and unsubscribing from it.
  Reflects the current code.

  Processes:
    - clients: sessions calling broker.Subscribe / broker.Unsubscribe for the stream
      (from handleCommandReply, TryRestoreSession, Node.Disconnect), in turn, any
      number of times. Different sessions don't share s.rmu, so calls are concurrent.
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
  psMap,       \* Subscriber.subscriptions has the stream (subMu)
  psClient,    \* RedisSubscriber.psClient: connection id (0 = nil) (psMu)
  redisConn,   \* live Redis pub/sub connection id (0 = none)
  redisSubs,   \* the live Redis connection is subscribed to the stream
  natsSubs,    \* live NATS subscriptions for the stream
  drops,       \* Redis connection drops so far
  cliConn,     \* clients: psClient read by Subscribe/Unsubscribe
  resubSnap,   \* ps: resubscribe snapshot of subscriptions has the stream
  pc

psVars    == <<psMap, psClient, redisConn, redisSubs, natsSubs, drops>>
localVars == <<cliConn, resubSnap>>
vars      == <<tracker, psVars, localVars, pc>>

Init ==
  /\ tracker = 0
  /\ psMap = FALSE
  /\ psClient = 1
  /\ redisConn = 1
  /\ redisSubs = FALSE
  /\ natsSubs = 0
  /\ drops = 0
  /\ cliConn = [c \in Clients |-> 0]
  /\ resubSnap = FALSE
  /\ pc = [p \in Procs |-> IF p = "ps" THEN "Live" ELSE "Idle"]

Goto(p, l) == pc' = [pc EXCEPT ![p] = l]

SubscriberSub == IF Adapter = "redis" THEN "RedisSubMap" ELSE "NatsSubChk"
SubscriberUnsub == IF Adapter = "redis" THEN "RedisUnsubMap" ELSE "NatsUnsubLock"

-----------------------------------------------------------------------------
(* broker *)

\* broker.Subscribe: isNew := tracker.Add(stream) (tracker.mu); broadcaster.Subscribe if new
BrokerSubAdd(c) ==
  /\ pc[c] = "Idle"
  /\ tracker' = tracker + 1
  /\ Goto(c, IF tracker = 0 THEN SubscriberSub ELSE "Held")
  /\ UNCHANGED <<psVars, localVars>>

\* broker.Unsubscribe: isLast := tracker.Remove(stream) (tracker.mu); broadcaster.Unsubscribe if last
BrokerUnsubRemove(c) ==
  /\ pc[c] = "Held"
  /\ tracker' = tracker - 1
  /\ Goto(c, IF tracker = 1 THEN SubscriberUnsub ELSE "Idle")
  /\ UNCHANGED <<psVars, localVars>>

-----------------------------------------------------------------------------
(* RedisSubscriber *)

\* redis.go: Subscribe — subscriptions[stream] = entry (subMu)
RedisSubMap(c) ==
  /\ pc[c] = "RedisSubMap"
  /\ psMap' = TRUE
  /\ Goto(c, "RedisSubClient")
  /\ UNCHANGED <<tracker, psClient, redisConn, redisSubs, natsSubs, drops, localVars>>

\* redis.go: Subscribe — client := s.pubsubClient() (psMu); return if nil
RedisSubClient(c) ==
  /\ pc[c] = "RedisSubClient"
  /\ cliConn' = [cliConn EXCEPT ![c] = psClient]
  /\ Goto(c, IF psClient = 0 THEN "Held" ELSE "RedisSubDo")
  /\ UNCHANGED <<tracker, psVars, resubSnap>>

\* redis.go: Subscribe — client.Do(SUBSCRIBE) (no effect on a dead connection)
RedisSubDo(c) ==
  /\ pc[c] = "RedisSubDo"
  /\ redisSubs' = IF cliConn[c] = redisConn THEN TRUE ELSE redisSubs
  /\ Goto(c, "Held")
  /\ UNCHANGED <<tracker, psMap, psClient, redisConn, natsSubs, drops, localVars>>

\* redis.go: Unsubscribe — return if not in subscriptions; delete(subscriptions, stream) (subMu)
RedisUnsubMap(c) ==
  /\ pc[c] = "RedisUnsubMap"
  /\ psMap' = FALSE
  /\ Goto(c, IF psMap THEN "RedisUnsubClient" ELSE "Idle")
  /\ UNCHANGED <<tracker, psClient, redisConn, redisSubs, natsSubs, drops, localVars>>

\* redis.go: Unsubscribe — client := s.pubsubClient() (psMu); return if nil
RedisUnsubClient(c) ==
  /\ pc[c] = "RedisUnsubClient"
  /\ cliConn' = [cliConn EXCEPT ![c] = psClient]
  /\ Goto(c, IF psClient = 0 THEN "Idle" ELSE "RedisUnsubDo")
  /\ UNCHANGED <<tracker, psVars, resubSnap>>

\* redis.go: Unsubscribe — client.Do(UNSUBSCRIBE) (no effect on a dead connection)
RedisUnsubDo(c) ==
  /\ pc[c] = "RedisUnsubDo"
  /\ redisSubs' = IF cliConn[c] = redisConn THEN FALSE ELSE redisSubs
  /\ Goto(c, "Idle")
  /\ UNCHANGED <<tracker, psMap, psClient, redisConn, natsSubs, drops, localVars>>

\* runPubSub: <-wait fires (connection lost)
ReconnDrop ==
  /\ Adapter = "redis"
  /\ pc["ps"] = "Live"
  /\ drops < MaxDrops
  /\ drops' = drops + 1
  /\ redisConn' = 0
  /\ redisSubs' = FALSE
  /\ Goto("ps", "ReconnNil")
  /\ UNCHANGED <<tracker, psMap, psClient, natsSubs, localVars>>

\* maybeReconnect: s.psClient = nil (psMu)
ReconnNil ==
  /\ pc["ps"] = "ReconnNil"
  /\ psClient' = 0
  /\ Goto("ps", "ReconnDial")
  /\ UNCHANGED <<tracker, psMap, redisConn, redisSubs, natsSubs, drops, localVars>>

\* runPubSub: s.client.Dedicate(); s.psClient = client (psMu)
ReconnDial ==
  /\ pc["ps"] = "ReconnDial"
  /\ redisConn' = drops + 1
  /\ psClient' = drops + 1
  /\ Goto("ps", "ResubSnap")
  /\ UNCHANGED <<tracker, psMap, redisSubs, natsSubs, drops, localVars>>

\* resubscribe: channels := maps.Keys(s.subscriptions) (subMu.RLock)
ResubSnap ==
  /\ pc["ps"] = "ResubSnap"
  /\ resubSnap' = psMap
  /\ Goto("ps", "ResubDo")
  /\ UNCHANGED <<tracker, psVars, cliConn>>

\* resubscribe: batchSubscribe(client, channels)
ResubDo ==
  /\ pc["ps"] = "ResubDo"
  /\ redisSubs' = IF resubSnap /\ psClient = redisConn THEN TRUE ELSE redisSubs
  /\ Goto("ps", "Live")
  /\ UNCHANGED <<tracker, psMap, psClient, redisConn, natsSubs, drops, localVars>>

-----------------------------------------------------------------------------
(* NATSSubscriber *)

\* nats.go: Subscribe — return if in subscriptions (subMu.RLock)
NatsSubChk(c) ==
  /\ pc[c] = "NatsSubChk"
  /\ Goto(c, IF psMap THEN "Held" ELSE "NatsSubLock")
  /\ UNCHANGED <<tracker, psVars, localVars>>

\* nats.go: Subscribe — conn.Subscribe; subscriptions[stream] = sub (subMu)
NatsSubLock(c) ==
  /\ pc[c] = "NatsSubLock"
  /\ natsSubs' = natsSubs + 1
  /\ psMap' = TRUE
  /\ Goto(c, "Held")
  /\ UNCHANGED <<tracker, psClient, redisConn, redisSubs, drops, localVars>>

\* nats.go: Unsubscribe — if in subscriptions: delete, sub.Unsubscribe() (subMu)
NatsUnsubLock(c) ==
  /\ pc[c] = "NatsUnsubLock"
  /\ psMap' = FALSE
  /\ natsSubs' = IF psMap THEN natsSubs - 1 ELSE natsSubs
  /\ Goto(c, "Idle")
  /\ UNCHANGED <<tracker, psClient, redisConn, redisSubs, drops, localVars>>

-----------------------------------------------------------------------------

Client(c) ==
  \/ BrokerSubAdd(c) \/ BrokerUnsubRemove(c)
  \/ RedisSubMap(c) \/ RedisSubClient(c) \/ RedisSubDo(c)
  \/ RedisUnsubMap(c) \/ RedisUnsubClient(c) \/ RedisUnsubDo(c)
  \/ NatsSubChk(c) \/ NatsSubLock(c) \/ NatsUnsubLock(c)

PubSub == ReconnDrop \/ ReconnNil \/ ReconnDial \/ ResubSnap \/ ResubDo

Next == (\E c \in Clients : Client(c)) \/ PubSub

Spec == Init /\ [][Next]_vars

Quiescent ==
  /\ \A c \in Clients : pc[c] \in {"Idle", "Held"}
  /\ pc["ps"] = "Live"

Holders ==
  {c \in Clients : pc[c] \in {"Held", "RedisSubMap", "RedisSubClient", "RedisSubDo", "NatsSubChk", "NatsSubLock"}}

-----------------------------------------------------------------------------

TypeOK ==
  /\ tracker \in 0..Cardinality(Clients)
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
