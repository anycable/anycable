# frozen_string_literal: true

require "redis"
require "json"

module Anycable
  # PubSub for broadcasting
  class PubSub
    attr_reader :redis_conn

    def initialize
      redis_config = { url: Anycable.config.redis_url }
      unless Anycable.config.redis_sentinels.empty?
        redis_config[:sentinels] = Anycable.config.redis_sentinels
      end
      @redis_conn = Redis.new(redis_config)
    end

    def broadcast(channel, payload)
      redis_conn.publish(
        Anycable.config.redis_channel,
        { stream: channel, data: payload }.to_json
      )
    end
  end
end
