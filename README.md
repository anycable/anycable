[![Gem Version](https://badge.fury.io/rb/anycable.svg)](https://rubygems.org/gems/anycable) [![Build Status](https://travis-ci.org/anycable/anycable.svg?branch=master)](https://travis-ci.org/anycable/anycable) [![Circle CI](https://circleci.com/gh/anycable/anycable/tree/master.svg?style=svg)](https://circleci.com/gh/anycable/anycable/tree/master)

# Anycable

AnyCable allows you to use any WebSocket server (written in any language) as a replacement for built-in Ruby ActionCable server.

With AnyCable you can use channels, client-side JS, broadcasting - (almost) all that you can do with ActionCable.

You can even use ActionCable in development and not be afraid of compatibility issues.

## Requirements

- Ruby ~> 2.3;
- Rails ~> 5.0;
- Redis

## How It Works?

TBD

## Compatible WebSocket servers

TBD


## Installation

Add Anycable to your application's Gemfile:

```ruby
gem 'anycable', group: :production
```

And then run:

```shell
rails generate anycable
```

to create executable.

You can use _built-in_ ActionCable for test and development.

## Configuration

Add `config/anycable.yml`if you want to override defaults (see below):

```yml
production:
  # gRPC server host and port
  rpc_host: "localhost:50051"
  # Redis URL (for broadcasting) 
  redis_url: "redis://localhost:6379/2"
  # Redis channel name
  redis_channel: "anycable"

```

Anycable uses [anyway_config](https://github.com/palkan/anyway_config), thus it is also possible to set configuration variables through `secrets.yml` or environment vars.

## Usage

Run Anycable server:

```ruby
bundle exec anycable
```

## ActionCable Compatibility


Feature                  | Status 
-------------------------|--------
Connection Identifiers   | +
Connection Request (cookies, params) | +
Disconnect Handling | coming soon
Subscribe to channels | +
Parameterized subscriptions | coming soon
Unsubscribe from channels | +
[Subscription Instance Variables](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
Performing Channel Actions | +
Streaming | +
[Custom stream callbacks](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
Broadcasting | +


## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable.

## License
The gem is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
