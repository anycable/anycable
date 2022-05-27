# frozen_string_literal: true

begin
  require "redis"
rescue LoadError
  raise "Please, install redis gem to use Redis broadcast adapter"
end

require "json"

module AnyCable
  module BroadcastAdapters
    # Redis adapter for broadcasting.
    #
    # Example:
    #
    #   AnyCable.broadast_adapter = :redis
    #
    # It uses Redis configuration from global AnyCable config
    # by default.
    #
    # You can override these params:
    #
    #   AnyCable.broadcast_adapter = :redis, url: "redis://my_redis", channel: "_any_cable_"
    class Redis < Base
      attr_reader :redis_conn, :channel

      def initialize(
        channel: AnyCable.config.redis_channel,
        **options
      )
        options = AnyCable.config.to_redis_params.merge(options)
        options[:driver] = :ruby
        @redis_conn = ::Redis.new(**options)
        @channel = channel
      end

      def raw_broadcast(payload)
        redis_conn.publish(channel, payload)
      end

      def announce!
        logger.info "Broadcasting Redis channel: #{channel}"
      end
    end
  end
end
