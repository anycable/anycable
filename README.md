[![GitPitch](https://gitpitch.com/assets/badge.svg)](https://gitpitch.com/anycable/anycable/master?grs=github) [![Gem Version](https://badge.fury.io/rb/anycable.svg)](https://rubygems.org/gems/anycable) [![Build Status](https://travis-ci.org/anycable/anycable.svg?branch=master)](https://travis-ci.org/anycable/anycable)
[![Gitter](https://img.shields.io/badge/gitter-join%20chat%20%E2%86%92-brightgreen.svg)](https://gitter.im/anycable/Lobby)

# AnyCable

AnyCable allows you to use any WebSocket server (written in any language) as a replacement for your Ruby server (such as Faye, ActionCable, etc).

AnyCable uses ActionCable protocol, so you can use ActionCable [JavaScript client](https://www.npmjs.com/package/actioncable) without any monkey-patching.

<a href="https://evilmartians.com/">
<img src="https://evilmartians.com/badges/sponsored-by-evil-martians.svg" alt="Sponsored by Evil Martians" width="236" height="54"></a>

## Requirements

- Ruby >= 2.4
- Redis (for brodcasting, [discuss other options](https://github.com/anycable/anycable/issues/2) with us!)

## How It Works?

![](https://s3.amazonaws.com/anycable/Scheme.png)

Read our [Wiki](https://github.com/anycable/anycable/wiki) for more.

## Links

- [AnyCable: Action Cable on steroids!](https://evilmartians.com/chronicles/anycable-actioncable-on-steroids)

- [Connecting LiteCable to Hanami](http://gabrielmalakias.com.br/ruby/hanami/iot/2017/05/26/websockets-connecting-litecable-to-hanami.html) by [@GabrielMalakias](https://github.com/GabrielMalakias)

- [From Action to Any](https://medium.com/@leshchuk/from-action-to-any-1e8d863dd4cf) by [@alekseyl](https://github.com/alekseyl)

## Talks

- One cable to rule them all, RubyKaigi 2018, [slides](https://speakerdeck.com/palkan/rubykaigi-2018-anycable-one-cable-to-rule-them-all) and [video](https://www.youtube.com/watch?v=jXCPuNICT8s) (EN)

- Wroc_Love.rb 2018 [slides](https://speakerdeck.com/palkan/wroc-love-dot-rb-2018-cables-cables-cables) and [video](https://www.youtube.com/watch?v=AUxFFOehiy0) (EN)

- RubyConfMY 2017 [slides](https://speakerdeck.com/palkan/rubyconf-malaysia-2017-anycable) and [video](https://www.youtube.com/watch?v=j5oFx525zNw) (EN)

- RailsClub Moscow 2016 [slides](https://speakerdeck.com/palkan/railsclub-moscow-2016-anycable) and [video](https://www.youtube.com/watch?v=-k7GQKuBevY&list=PLiWUIs1hSNeOXZhotgDX7Y7qBsr24cu7o&index=4) (RU)


## Compatible WebSocket servers

- [Anycable Go](https://github.com/anycable/anycable-go)
- [ErlyCable](https://github.com/anycable/erlycable)

## Configuration

Anycable uses [anyway_config](https://github.com/palkan/anyway_config), thus it is also possible to set configuration variables through `secrets.yml` or environment vars.

### Example with Redis Sentinel

```yaml
  rpc_host: "localhost:50123"
  redis_url: "redis://redis-1-1:6379/2"
  redis_sentinels:
    - { host: 'redis-1-1', port: 26379 }
    - { host: 'redis-1-2', port: 26379 }
    - { host: 'redis-1-3', port: 26379 }
```

## Action Cable Compatibility

This is the compatibility list for the AnyCable gem, not for AnyCable servers (which may not support some of the features yet).

Feature                  | Status
-------------------------|--------
Connection Identifiers   | +
Connection Request (cookies, params) | +
Disconnect Handling | +
Subscribe to channels | +
Parameterized subscriptions | +
Unsubscribe from channels | +
[Subscription Instance Variables](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
Performing Channel Actions | +
Streaming | +
[Custom stream callbacks](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
Broadcasting | +
Custom pubsub adapter | Only redis

## Build

- Install required GRPC gems:

```
gem install grpc
gem install grpc-tools
```

- Re-generate GRPC files (if necessary):

```
make
```

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable.

Please, provide reproduction script (using [this template](https://github.com/anycable/anycable/blob/master/etc/bug_report_template.rb)) when submitting bugs if possible.

## License
The gem is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
