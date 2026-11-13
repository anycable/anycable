# TLA+ models

Specs are written in plain TLA+ (no PlusCal) and checked with the bundled [manyan](https://github.com/soasme/manyan):

```sh
./manyan -config SessionLifecycle.cfg SessionLifecycle.tla
./manyan -config BrokerPubSub.cfg BrokerPubSub.tla
```

`BrokerPubSub.cfg` checks the Redis subscriber; set `Adapter = "nats"` for NATS.

manyan stops at the first violated invariant. To check a single invariant, use a config with only that `INVARIANT`.

Add `-difftrace` to print only the changed variables at each step of a counterexample.
