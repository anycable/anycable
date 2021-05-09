# Apollo GraphQL protocol support <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable can act as a _translator_ between [Apollo GraphQL][apollo-protocol] and Action Cable protocols (used by [GraphQL Ruby][graphql-ruby]).

That allows us to use the variety of tools compatible with Apollo: client-side libraries, IDEs (such as Apollo Studio).

## Usage

Run `anycable-go` with `apollo_path` option specified as follows:

```sh
$ anycable-go -apollo_path=/graphql

 ...
 INFO 2021-05-11T11:53:01.186Z context=main Handle Apollo GraphQL WebSocket connections at http://localhost:8080/graphql
 ...

# or using env var
$ APOLLO_GRAPHQL_PATH=/graphql anycable-go
```

Now your Apollo clients can connect to the `/graphql` endpoint to consume your GraphQL API.

GraphQL Ruby code stays unchanged (make sure you use [graphql-anycable][] plugin).

Other configuration options:

**--apollo_channel** (`ANYCABLE_APOLLO_CHANNEL`)

GraphQL Ruby channel class name (default: `"GraphqlChannel"`).

**--apollo_action** (`ANYCABLE_APOLLO_ACTION`)

GraphQL Ruby channel action name (default: `"execute"`).

[apollo-protocol]: https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#graphql-over-websocket-protocol
[graphql-ruby]: https://graphql-ruby.org
[graphql-anycable]: https://github.com/anycable/graphql-anycable
