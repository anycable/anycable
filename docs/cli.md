# AnyCable CLI

[AnyCable gem](https://github.com/anycable/anycable) consists of a gRPC server implementation and a CLI to run this server along with your Ruby application.

Run `anycable` CLI to start a gRPC server:

```sh
$ bundle exec anycable --require "./path/to/app.rb"
#> Starting AnyCable gRPC server (pid: 85746, workers_num: 30)
#> AnyCable version: 1.1.0
#> gRPC version: 1.37.0
#> Serving Rails application from ./path/to/app.rb ...
#> ...
```

You only have to tell AnyCable where to find your application code.

**NOTE:** AnyCable tries to detect where to load your app from if no `--require` option is provided.
It checks for `config/anycable.rb` and `config/environment.rb` files presence (in the specified order).

Run `anycable -h` to see the list of all available options and their defaults.

## Running WebSocket server along with RPC

AnyCable CLI provides an option to run any arbitrary command along with the RPC server. That could be useful for local development and even in production (e.g. for [Heroku deployment](../deployment/heroku.md)).

For example:

```sh
$ bundle exec anycable --server-command "anycable-go -p 8080"
#> Starting AnyCable gRPC server (pid: 85746, workers_num: 30)
#> AnyCable version: 1.1.0
#> gRPC version: 1.37.0
#> Serving Rails application from ./path/to/app.rb ...
#> ...
#> Started command: anycable-go --port 8080 (pid: 13710)
#> ...
```
