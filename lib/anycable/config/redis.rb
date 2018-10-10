# frozen_string_literal: true

require "anyway_config"

module Anycable
  # Redis adapter configuration.
  class RedisConfig < Anyway::Config
    # TODO: upgrade anyway_config
    env_prefix :anycable_redis
    config_name :anycable_redis

    # TODO: add feature to anyway_config
    # config_file :anycable, key: [:pubsub, :options]

    attr_config(
      url: ENV.fetch("REDIS_URL", "redis://localhost:6379/5"),
      sentinels: [],
      channel: "__anycable__"
    )

    # Build Redis instance initialization parameters
    def to_redis_params
      { url: url }.tap do |params|
        params[:sentinels] = sentinels unless sentinels.empty?
      end
    end
  end
end
