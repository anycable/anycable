--------------------------- MODULE SessionLifecycle ---------------------------
(*
  A single session's lifecycle racing with asynchronous closes.
  Reflects the current code.

  Processes:
    - reader: the session's read loop. Commands are handled sequentially:
      Node.Authenticate, then Node.Subscribe (handleCommandReply with reply.Streams),
      then (optionally) Node.Unsubscribe (handleCommandReply with StopAllStreams).
    - c1: Session.Disconnect (ping/pong timeouts, remote disconnect, command failure):
      close() -> disconnectFromNode() -> Node.Disconnect.
    - c2: Session.disconnectNow (writer: write failure, close frame sent):
      connected.Load() check -> close() -> disconnectFromNode() -> Node.Disconnect.
    Each closer may never fire.
*)
EXTENDS Integers, FiniteSets, TLC

CONSTANT Streams  \* streams returned by the channel's subscribe RPC

SID == "sid"
NoStream == "none"
Closers == {"c1", "c2"}
Procs == {"reader"} \cup Closers

VARIABLES
  closed,            \* Session.closed (s.mu)
  connected,         \* Session.connected (s.rmu)
  subsChan,          \* Session.subscriptions has the channel
  subsStreams,       \* Session.subscriptions channel streams
  rmu,               \* Session.rmu holder ("none" or a process)
  hubSessions,      \* Hub.sessions keys (h.mu)
  hubStreams,        \* Hub.sessions[sid].streams (h.mu)
  hubGate,           \* Gate.streams[*][session] (g.mu)
  tracker,           \* broker StreamsTracker (never goes below 0)
  pubsub,            \* Subscriber.subscriptions
  refs,              \* ghost: references taken minus released by this session, per stream
  rpcResult,         \* reader: last RPC result
  todoSubStreams,    \* reader: reply.Streams left to subscribe
  curStream,         \* reader: stream being processed
  todoUnsubStreams,  \* reader: streams returned by RemoveChannelStreams, left to release
  subsSnapshot,      \* closers: snapshot of subscriptions (per closer)
  pc                 \* program counters

sessVars   == <<closed, connected, subsChan, subsStreams>>
hubVars    == <<hubSessions, hubStreams, hubGate>>
brokerVars == <<tracker, pubsub, refs>>
readerVars == <<rpcResult, todoSubStreams, curStream, todoUnsubStreams>>
vars       == <<sessVars, rmu, hubVars, brokerVars, readerVars, subsSnapshot, pc>>

Init ==
  /\ closed = FALSE
  /\ connected = FALSE
  /\ subsChan = FALSE
  /\ subsStreams = {}
  /\ rmu = "none"
  /\ hubSessions = {}
  /\ hubStreams = {}
  /\ hubGate = {}
  /\ tracker = [s \in Streams |-> 0]
  /\ pubsub = {}
  /\ refs = [s \in Streams |-> 0]
  /\ rpcResult = "none"
  /\ todoSubStreams = {}
  /\ curStream = NoStream
  /\ todoUnsubStreams = {}
  /\ subsSnapshot = [c \in Closers |-> {}]
  /\ pc = [p \in Procs |-> "Start"]

Goto(p, l) == pc' = [pc EXCEPT ![p] = l]

\* broker.Subscribe: tracker.Add (+ broadcaster.Subscribe if new)
BrokerSubscribe(s) ==
  /\ tracker' = [tracker EXCEPT ![s] = @ + 1]
  /\ pubsub' = IF tracker[s] = 0 THEN pubsub \cup {s} ELSE pubsub
  /\ refs' = [refs EXCEPT ![s] = @ + 1]

\* broker.Unsubscribe: tracker.Remove (+ broadcaster.Unsubscribe if last);
\* Remove is a no-op for unknown streams
BrokerUnsubscribe(s) ==
  /\ tracker' = [tracker EXCEPT ![s] = IF @ > 0 THEN @ - 1 ELSE 0]
  /\ pubsub' = IF tracker[s] = 1 THEN pubsub \ {s} ELSE pubsub
  /\ refs' = [refs EXCEPT ![s] = @ - 1]

-----------------------------------------------------------------------------
(* reader: Authenticate *)

\* node.go: Authenticate -> controller.Authenticate (RPC)
AuthRPC ==
  /\ pc["reader"] = "Start"
  /\ rpcResult' \in {"success", "failure"}
  /\ Goto("reader", "AuthRPCChk")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, todoSubStreams, curStream, todoUnsubStreams, subsSnapshot, rmu>>

\* node.go: if s.IsClosed() { return }
AuthRPCChk ==
  /\ pc["reader"] = "AuthRPCChk"
  /\ Goto("reader", IF closed \/ rpcResult /= "success" THEN "Done" ELSE "Authenticated")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Authenticated() under s.rmu
Authenticated ==
  /\ pc["reader"] = "Authenticated"
  /\ rmu = "none"
  /\ IF closed
       THEN /\ Goto("reader", "Done")
            /\ UNCHANGED <<connected, hubSessions>>
       ELSE /\ hubSessions' = hubSessions \cup {SID}
            /\ connected' = TRUE
            /\ Goto("reader", "SubCmd")
  /\ UNCHANGED <<closed, subsChan, subsStreams, hubStreams, hubGate, brokerVars, readerVars, subsSnapshot, rmu>>

-----------------------------------------------------------------------------
(* reader: Subscribe *)

\* session.go: Serve loop, if s.IsClosed() { return } before reading a command
SubCmd ==
  /\ pc["reader"] = "SubCmd"
  /\ Goto("reader", IF closed THEN "Done" ELSE "SubRPC")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Subscribe -> controller.Subscribe (RPC in flight)
SubRPC ==
  /\ pc["reader"] = "SubRPC"
  /\ Goto("reader", "SubRPCChk")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: if s.IsClosed() { return } ; subscriptions.AddChannel (s.smu)
SubRPCChk ==
  /\ pc["reader"] = "SubRPCChk"
  /\ IF closed
       THEN /\ Goto("reader", "Done")
            /\ UNCHANGED <<subsChan, subsStreams, todoSubStreams>>
       ELSE /\ subsChan' = TRUE
            /\ subsStreams' = {}
            /\ todoSubStreams' = Streams
            /\ Goto("reader", "SubReplyLock")
  /\ UNCHANGED <<closed, connected, hubVars, brokerVars, rpcResult, curStream, todoUnsubStreams, subsSnapshot, rmu>>

\* node.go: Subscribe — s.rmu.Lock(); if s.IsClosed() { s.rmu.Unlock(); return }
SubReplyLock ==
  /\ pc["reader"] = "SubReplyLock"
  /\ rmu = "none"
  /\ IF closed
       THEN /\ Goto("reader", "ServeLoop")
            /\ UNCHANGED rmu
       ELSE /\ rmu' = "reader"
            /\ Goto("reader", "ReplyStreamsLoop")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

\* node.go: Subscribe — s.rmu.Unlock() after handleCommandReply
SubReplyUnlock ==
  /\ pc["reader"] = "SubReplyUnlock"
  /\ rmu' = "none"
  /\ Goto("reader", "ServeLoop")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

\* node.go: handleCommandReply, reply.Streams loop
ReplyStreamsLoop ==
  /\ pc["reader"] = "ReplyStreamsLoop"
  /\ IF todoSubStreams = {}
       THEN /\ Goto("reader", "SubReplyUnlock")
            /\ UNCHANGED curStream
       ELSE /\ curStream' = CHOOSE x \in todoSubStreams : TRUE
            /\ Goto("reader", "BrokerSub")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, rpcResult, todoSubStreams, todoUnsubStreams, subsSnapshot, rmu>>

\* node.go: broker.Subscribe
BrokerSub ==
  /\ pc["reader"] = "BrokerSub"
  /\ BrokerSubscribe(curStream)
  /\ Goto("reader", "HubSub")
  /\ UNCHANGED <<sessVars, hubVars, readerVars, subsSnapshot, rmu>>

\* hub.go: SubscribeSession (h.mu) — return if the session is missing; gate.Subscribe, AddStream
HubSub ==
  /\ pc["reader"] = "HubSub"
  /\ IF SID \in hubSessions
       THEN /\ hubGate' = hubGate \cup {curStream}
            /\ hubStreams' = hubStreams \cup {curStream}
       ELSE UNCHANGED <<hubGate, hubStreams>>
  /\ Goto("reader", "StateAddStream")
  /\ UNCHANGED <<sessVars, hubSessions, brokerVars, readerVars, subsSnapshot, rmu>>

\* subscription_state.go: AddChannelStream (no-op if the channel is missing)
StateAddStream ==
  /\ pc["reader"] = "StateAddStream"
  /\ subsStreams' = IF subsChan THEN subsStreams \cup {curStream} ELSE subsStreams
  /\ todoSubStreams' = todoSubStreams \ {curStream}
  /\ Goto("reader", "ReplyStreamsLoop")
  /\ UNCHANGED <<closed, connected, subsChan, hubVars, brokerVars, rpcResult, curStream, todoUnsubStreams, subsSnapshot, rmu>>

-----------------------------------------------------------------------------
(* reader: Unsubscribe *)

\* session.go: Serve loop, if s.IsClosed() { return }; the client may send no more commands
ServeLoop ==
  /\ pc["reader"] = "ServeLoop"
  /\ IF closed
       THEN Goto("reader", "Done")
       ELSE \E l \in {"Done", "UnsubHasChan"} : Goto("reader", l)
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Unsubscribe — subscriptions.HasChannel (s.smu)
UnsubHasChan ==
  /\ pc["reader"] = "UnsubHasChan"
  /\ Goto("reader", IF subsChan THEN "UnsubRPC" ELSE "Done")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Unsubscribe -> controller.Unsubscribe (RPC); an error returns early
UnsubRPC ==
  /\ pc["reader"] = "UnsubRPC"
  /\ rpcResult' \in {"success", "error"}
  /\ Goto("reader", "UnsubRPCChk")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, todoSubStreams, curStream, todoUnsubStreams, subsSnapshot, rmu>>

\* node.go: if s.IsClosed() { return }; on error, return
UnsubRPCChk ==
  /\ pc["reader"] = "UnsubRPCChk"
  /\ Goto("reader", IF closed \/ rpcResult /= "success" THEN "Done" ELSE "StopReplyLock")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* handleCommandReply — s.rmu.Lock(); if s.IsClosed() { s.rmu.Unlock(); return }
StopReplyLock ==
  /\ pc["reader"] = "StopReplyLock"
  /\ rmu = "none"
  /\ IF closed
       THEN /\ Goto("reader", "StateRemoveChanChk")
            /\ UNCHANGED rmu
       ELSE /\ rmu' = "reader"
            /\ Goto("reader", "StopHubUnsub")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

\* handleCommandReply — s.rmu.Unlock()
StopReplyUnlock ==
  /\ pc["reader"] = "StopReplyUnlock"
  /\ rmu' = "none"
  /\ Goto("reader", "StateRemoveChanChk")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

\* node.go: Unsubscribe — if err == nil && !s.IsClosed() (before RemoveChannel)
StateRemoveChanChk ==
  /\ pc["reader"] = "StateRemoveChanChk"
  /\ Goto("reader", IF closed THEN "Done" ELSE "StateRemoveChan")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* handleCommandReply, StopAllStreams: hub.UnsubscribeSessionFromChannel (h.mu + gate.Unsubscribe)
StopHubUnsub ==
  /\ pc["reader"] = "StopHubUnsub"
  /\ IF SID \in hubSessions
       THEN /\ hubGate' = hubGate \ hubStreams
            /\ hubStreams' = {}
       ELSE UNCHANGED <<hubGate, hubStreams>>
  /\ Goto("reader", "StopStateRemoveStreams")
  /\ UNCHANGED <<sessVars, hubSessions, brokerVars, readerVars, subsSnapshot, rmu>>

\* handleCommandReply, StopAllStreams: subscriptions.RemoveChannelStreams
StopStateRemoveStreams ==
  /\ pc["reader"] = "StopStateRemoveStreams"
  /\ todoUnsubStreams' = IF subsChan THEN subsStreams ELSE {}
  /\ subsStreams' = {}
  /\ Goto("reader", "StopBrokerUnsubLoop")
  /\ UNCHANGED <<closed, connected, subsChan, hubVars, brokerVars, rpcResult, todoSubStreams, curStream, subsSnapshot, rmu>>

\* handleCommandReply, StopAllStreams: broker.Unsubscribe for each removed stream
StopBrokerUnsubLoop ==
  /\ pc["reader"] = "StopBrokerUnsubLoop"
  /\ IF todoUnsubStreams = {}
       THEN /\ Goto("reader", "StopReplyUnlock")
            /\ UNCHANGED <<brokerVars, todoUnsubStreams>>
       ELSE \E s \in todoUnsubStreams :
              /\ BrokerUnsubscribe(s)
              /\ todoUnsubStreams' = todoUnsubStreams \ {s}
              /\ UNCHANGED pc
  /\ UNCHANGED <<sessVars, hubVars, rpcResult, todoSubStreams, curStream, subsSnapshot, rmu>>

\* node.go: Unsubscribe — subscriptions.RemoveChannel
StateRemoveChan ==
  /\ pc["reader"] = "StateRemoveChan"
  /\ subsChan' = FALSE
  /\ subsStreams' = {}
  /\ Goto("reader", "Done")
  /\ UNCHANGED <<closed, connected, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

Reader ==
  \/ AuthRPC \/ AuthRPCChk \/ Authenticated
  \/ SubCmd \/ SubRPC \/ SubRPCChk \/ SubReplyLock \/ ReplyStreamsLoop \/ BrokerSub \/ HubSub
  \/ StateAddStream \/ SubReplyUnlock
  \/ ServeLoop \/ UnsubHasChan \/ UnsubRPC \/ UnsubRPCChk \/ StopReplyLock \/ StopHubUnsub \/ StopStateRemoveStreams
  \/ StopBrokerUnsubLoop \/ StopReplyUnlock \/ StateRemoveChanChk \/ StateRemoveChan

-----------------------------------------------------------------------------
(* closers *)

\* The closer fires (c2 first checks connected.Load()) or never fires
CloserStart(c) ==
  /\ pc[c] = "Start"
  /\ \/ Goto(c, "Done")
     \/ /\ c = "c1"
        /\ Goto(c, "SessionClose")
     \/ /\ c = "c2"
        /\ Goto(c, IF connected THEN "SessionClose" ELSE "Done")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* session.go: close()
SessionClose(c) ==
  /\ pc[c] = "SessionClose"
  /\ closed' = TRUE
  /\ Goto(c, "DisconnectConnected")
  /\ UNCHANGED <<connected, subsChan, subsStreams, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* session.go: disconnectFromNode() — connected.Swap(false) under s.rmu
DisconnectConnected(c) ==
  /\ pc[c] = "DisconnectConnected"
  /\ rmu = "none"
  /\ connected' = FALSE
  /\ Goto(c, IF connected THEN "DisconnectLock" ELSE "Done")
  /\ UNCHANGED <<closed, subsChan, subsStreams, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Disconnect — s.rmu.Lock()
DisconnectLock(c) ==
  /\ pc[c] = "DisconnectLock"
  /\ rmu = "none"
  /\ rmu' = c
  /\ Goto(c, "UnsubSnapStreams")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

\* node.go: unsubscribeSessionStreams — subscriptions.ToMap snapshot (s.smu)
UnsubSnapStreams(c) ==
  /\ pc[c] = "UnsubSnapStreams"
  /\ subsSnapshot' = [subsSnapshot EXCEPT ![c] = IF subsChan THEN subsStreams ELSE {}]
  /\ Goto(c, "UnsubBrokerUnsubLoop")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, rmu>>

\* node.go: unsubscribeSessionStreams — broker.Unsubscribe for each snapshotted stream
UnsubBrokerUnsubLoop(c) ==
  /\ pc[c] = "UnsubBrokerUnsubLoop"
  /\ IF subsSnapshot[c] = {}
       THEN /\ Goto(c, "HubRemoveSesChk")
            /\ UNCHANGED <<brokerVars, subsSnapshot>>
       ELSE \E s \in subsSnapshot[c] :
              /\ BrokerUnsubscribe(s)
              /\ subsSnapshot' = [subsSnapshot EXCEPT ![c] = @ \ {s}]
              /\ UNCHANGED pc
  /\ UNCHANGED <<sessVars, hubVars, readerVars, rmu>>

\* hub.go: RemoveSession — RLock check
HubRemoveSesChk(c) ==
  /\ pc[c] = "HubRemoveSesChk"
  /\ Goto(c, IF SID \in hubSessions THEN "HubRemoveSesUnsub" ELSE "DisconnectUnlock")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot, rmu>>

\* hub.go: unsubscribeSessionFromAllChannels (h.mu + gate.Unsubscribe)
HubRemoveSesUnsub(c) ==
  /\ pc[c] = "HubRemoveSesUnsub"
  /\ IF SID \in hubSessions
       THEN hubGate' = hubGate \ hubStreams
       ELSE UNCHANGED hubGate
  /\ Goto(c, "HubRemoveSesDel")
  /\ UNCHANGED <<sessVars, hubSessions, hubStreams, brokerVars, readerVars, subsSnapshot, rmu>>

\* hub.go: RemoveSession — delete under h.mu
HubRemoveSesDel(c) ==
  /\ pc[c] = "HubRemoveSesDel"
  /\ hubSessions' = hubSessions \ {SID}
  /\ hubStreams' = {}
  /\ Goto(c, "DisconnectUnlock")
  /\ UNCHANGED <<sessVars, hubGate, brokerVars, readerVars, subsSnapshot, rmu>>

\* node.go: Disconnect — s.rmu.Unlock()
DisconnectUnlock(c) ==
  /\ pc[c] = "DisconnectUnlock"
  /\ rmu' = "none"
  /\ Goto(c, "Done")
  /\ UNCHANGED <<sessVars, hubVars, brokerVars, readerVars, subsSnapshot>>

Closer(c) ==
  \/ CloserStart(c) \/ SessionClose(c) \/ DisconnectConnected(c) \/ DisconnectLock(c)
  \/ UnsubSnapStreams(c) \/ UnsubBrokerUnsubLoop(c)
  \/ HubRemoveSesChk(c) \/ HubRemoveSesUnsub(c) \/ HubRemoveSesDel(c) \/ DisconnectUnlock(c)

-----------------------------------------------------------------------------

Quiescent == \A p \in Procs : pc[p] = "Done"

Terminating == Quiescent /\ UNCHANGED vars

Next == Reader \/ (\E c \in Closers : Closer(c)) \/ Terminating

Spec == Init /\ [][Next]_vars

-----------------------------------------------------------------------------

TypeOK ==
  /\ closed \in BOOLEAN
  /\ connected \in BOOLEAN
  /\ subsChan \in BOOLEAN
  /\ subsStreams \subseteq Streams
  /\ rmu \in {"none"} \cup Procs
  /\ hubSessions \subseteq {SID}
  /\ hubStreams \subseteq Streams
  /\ hubGate \subseteq Streams
  /\ tracker \in [Streams -> Nat]
  /\ pubsub \subseteq Streams
  /\ refs \in [Streams -> Int]

\* The session never releases a broker reference it doesn't hold
\* (with other sessions on the same stream, that would steal their reference)
NoOverRelease == \A s \in Streams : refs[s] >= 0

\* A closed session is not tracked by the hub (no registration, no gate subscriptions)
HubClean ==
  (Quiescent /\ closed) =>
    /\ hubSessions = {}
    /\ hubStreams = {}
    /\ hubGate = {}

\* A closed session holds no broker references (and, thus, no pub/sub subscriptions)
BrokerClean ==
  (Quiescent /\ closed) =>
    /\ \A s \in Streams : refs[s] = 0
    /\ \A s \in Streams : tracker[s] = 0
    /\ pubsub = {}

\* An open session's hub and broker state matches its subscriptions
OpenConsistent ==
  (Quiescent /\ ~closed) =>
    /\ connected => hubSessions = {SID}
    /\ hubStreams = subsStreams
    /\ hubGate = subsStreams
    /\ \A s \in Streams : refs[s] = IF s \in subsStreams THEN 1 ELSE 0
    /\ pubsub = subsStreams

=============================================================================
