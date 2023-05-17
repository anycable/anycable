# frozen_string_literal: true

require "anycable/broadcast_adapters/redis"

module AnyCable
  module BroadcastAdapters
    # Next-gen Redis adapter for broadcasting over Redis streams.
    #
    # Unlike Redis adapter, RedisX adapter delivers each broadcast message only
    # to a single WS server, which is responsible for re-broadcasting it within the cluster.
    # It's required for the Broker (hot streams cache) support.
    #
    # Example:
    #
    #   AnyCable.broadcast_adapter = :redisx
    #
    # It uses Redis configuration from global AnyCable config
    # by default.
    # NOTE: The `redis_channel` config param is used as a stream name.
    #
    # You can override these params:
    #
    #   AnyCable.broadcast_adapter = :redisx, { url: "redis://my_redis", stream_name: "_any_cable_" }
    class Redisx < Redis
      def raw_broadcast(payload)
        redis_conn.xadd(channel, {payload: payload})
      end

      def announce!
        logger.info "Broadcasting Redis stream: #{channel}"
      end
    end
  end
end
