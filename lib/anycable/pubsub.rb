# frozen_string_literal: true

require "redis"
require "json"

require "anycable/config/redis"

module Anycable
  # PubSub for broadcasting
  class PubSub
    attr_reader :redis_conn, :config

    def initialize(config = RedisConfig.new)
      @config = config
      @redis_conn = Redis.new(config.to_redis_params)
    end

    def broadcast(channel, payload)
      redis_conn.publish(
        config.channel,
        { stream: channel, data: payload }.to_json
      )
    end
  end
end
