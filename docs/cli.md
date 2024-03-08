# AnyCable CLI

[AnyCable Ruby](https://github.com/anycable/anycable) comes with a gRPC server for AnyCable and a CLI to run this server along with your Ruby application.

Run `anycable` CLI to start a gRPC server:

```sh
$ bundle exec anycable --require "./path/to/app.rb"
#> Starting AnyCable gRPC server (pid: 85746, workers_num: 30)
#> AnyCable version: 1.5.0
#> gRPC version: 1.57.0
#> Serving Rails application from ./path/to/app.rb ...
#> ...
```

You only have to tell AnyCable where to find your application code via the `--require` (`-r`) option. By default, we check for the `config/anycable.rb` and `config/environment.rb` files presence (in the specified order). Thus, you don't need to specify any options when using with Ruby on Rails.

Run `anycable -h` to see the list of all available options and their defaults. See also [configuration documentation](./configuration.md).

## Running AnyCable server along with RPC

AnyCable CLI provides an option to run any arbitrary command along with the RPC server. That could be useful for local development and even in production (e.g. for [Heroku deployment](../deployment/heroku.md)).

For example:

```sh
$ bundle exec anycable --server-command "anycable-go -p 8080"
#> Starting AnyCable gRPC server (pid: 85746, workers_num: 30)
#> AnyCable version: 1.5.0
#> gRPC version: 1.57.0
#> Serving Rails application from ./path/to/app.rb ...
#> ...
#> Started command: anycable-go --port 8080 (pid: 13710)
#> ...
```
