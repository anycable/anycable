# Apollo GraphQL support <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable can act as a _translator_ between Apollo GraphQL and Action Cable protocols (used by [GraphQL Ruby][graphql-ruby]).

That allows us to use the variety of tools compatible with Apollo: client-side libraries, IDEs (such as Apollo Studio).

> See also the [demo](https://github.com/anycable/anycable_rails_demo/pull/18) of using AnyCable with Apollo GraphQL React Native application.

## Usage

Run `anycable-go` with `graphql_path` option specified as follows:

```sh
$ anycable-go -graphql_path=/graphql

 ...
 INFO 2021-05-11T11:53:01.186Z context=main Handle GraphQL WebSocket connections at http://localhost:8080/graphql
 ...

# or using env var
$ ANYCABLE_GRAPHQL_PATH=/graphql anycable-go
```

Now your Apollo-compatible\* GraphQL clients can connect to the `/graphql` endpoint to consume your GraphQL API.

\* Currently, there are two protocol implementations supported by AnyCable: [graphql-ws][] and (legacy) [subscriptions-transport-ws][].

GraphQL Ruby code stays unchanged (make sure you use [graphql-anycable][] plugin).

Other configuration options:

**--graphql_channel** (`ANYCABLE_GRAPHQL_CHANNEL`)

GraphQL Ruby channel class name (default: `"GraphqlChannel"`).

**--graphql_action** (`ANYCABLE_GRAPHQL_ACTION`)

GraphQL Ruby channel action name (default: `"execute"`).

## Client configuration

We test our implementation against the official Apollo WebSocket link configuration described here: [Get real-time updates from your GraphQL server][apollo-subscriptions].

## Authentication

Apollo GraphQL supports passing additional connection params during the connection establishment. For example:

```js
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { createClient } from 'graphql-ws';

const wsLink = new GraphQLWsLink(createClient({
  url: 'ws://localhost:8080/graphql',
  connectionParams: {
    token: 'some-token',
  },
}));
```

AnyCable passes these params via the `x-apollo-connection` HTTP header, which you can access in your `ApplicationCable::Connection#connect` method:

```ruby
module ApplicationCable
  class Connection < ActionCable::Connection::Base
    identified_by :user

    def connect
      user = find_user
      reject_unauthorized_connection unless user

      self.user = user
    end

    private

    def find_user
      header = request.headers["x-apollo-connection"]
      return unless header

      # Header contains JSON-encoded params
      payload = JSON.parse(header)

      User.find_by_token(payload["token"])
    end
  end
end
```

Note that the header contains JSON-encoded connection params object.

### Using with JWT identification

You can use [JWT identification](./jwt_identification.md) along with Apollo integration by specifying the token either via query params (e.g., `ws://localhost:8080/graphql?jid=<token>`) or by passing it along with connection params like this:

```js
const wsLink = new GraphQLWsLink(createClient({
  url: 'ws://localhost:8080/graphql',
  connectionParams: {
    jid: '<token>',
  },
}));
```

[subscriptions-transport-ws]: https://github.com/apollographql/subscriptions-transport-ws
[apollo-subscriptions]: https://www.apollographql.com/docs/react/data/subscriptions/
[graphql-ruby]: https://graphql-ruby.org
[graphql-anycable]: https://github.com/anycable/graphql-anycable
[graphql-ws]: https://github.com/enisdenjo/graphql-ws
