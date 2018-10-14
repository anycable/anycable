# frozen_string_literal: true

require "redis"
require "json"

module Anycable
  # PubSub for broadcasting
  class PubSub
    attr_reader :redis_conn

    def initialize
      @redis_conn = Redis.new(Anycable.config.to_redis_params)
    end

    def broadcast(channel, payload)
      redis_conn.publish(
        Anycable.config.redis_channel,
        { stream: channel, data: payload }.to_json
      )
    end
  end
end
