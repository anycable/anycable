# frozen_string_literal: true

gem "redis", ">= 3"

require "redis"
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
    class Redis
      attr_reader :redis_conn, :channel

      def initialize(
        channel: AnyCable.config.redis_channel,
        **options
      )
        options = AnyCable.config.to_redis_params.merge(options)
        @redis_conn = ::Redis.new(options)
        @channel = channel
      end

      def broadcast(stream, payload)
        redis_conn.publish(
          channel,
          { stream: stream, data: payload }.to_json
        )
      end
    end
  end
end
