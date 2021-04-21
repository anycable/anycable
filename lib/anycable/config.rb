# frozen_string_literal: true

require "anyway_config"

require "uri"

module AnyCable
  # AnyCable configuration.
  class Config < Anyway::Config
    class << self
      # Add usage txt for CLI
      def usage(txt)
        usages << txt
      end

      def usages
        @usages ||= []
      end
    end

    config_name :anycable

    attr_config(
      ## PubSub
      broadcast_adapter: :redis,

      ### Redis options
      redis_url: ENV.fetch("REDIS_URL", "redis://localhost:6379/5"),
      redis_sentinels: nil,
      redis_channel: "__anycable__",

      ### HTTP broadcasting options
      http_broadcast_url: "http://localhost:8090/_broadcast",
      http_broadcast_secret: nil,

      ### Logging options
      log_file: nil,
      log_level: :info,
      debug: false, # Shortcut to enable GRPC logging and debug level

      ### Health check options
      http_health_port: nil,
      http_health_path: "/health",

      ### Misc options
      version_check_enabled: true
    )

    alias_method :version_check_enabled?, :version_check_enabled

    flag_options :debug

    on_load { self.debug = debug != false }

    def log_level
      debug? ? :debug : super
    end

    def http_health_port_provided?
      !http_health_port.nil? && http_health_port != ""
    end

    usage <<~TXT
      APPLICATION
          --broadcast-adapter=type          Pub/sub adapter type for broadcasts, default: redis
          --log-level=level                 Logging level, default: "info"
          --log-file=path                   Path to log file, default: <none> (log to STDOUT)
          --debug                           Turn on verbose logging ("debug" level and gRPC logging on)

      HTTP HEALTH CHECKER
          --http-health-port=port           Port to run HTTP health server on, default: <none> (disabled)
          --http-health-path=path           Endpoint to server health cheks, default: "/health"

      REDIS PUB/SUB
          --redis-url=url                   Redis URL for pub/sub, default: REDIS_URL or "redis://localhost:6379/5"
          --redis-channel=name              Redis channel for broadcasting, default: "__anycable__"
          --redis-sentinels=<...hosts>      Redis Sentinel followers addresses (as a comma-separated list), default: nil

      HTTP PUB/SUB
          --http-broadcast-url              HTTP pub/sub endpoint URL, default: "http://localhost:8090/_broadcast"
          --http-broadcast-secret           HTTP pub/sub authorization secret, default: <none> (disabled)
    TXT

    # Build Redis parameters
    def to_redis_params
      {url: redis_url}.tap do |params|
        next if redis_sentinels.nil? || redis_sentinels.empty?

        sentinels = Array(redis_sentinels)

        next if sentinels.empty?

        params[:sentinels] = sentinels.map(&method(:parse_sentinel))
      end.tap do |params|
        next unless redis_url.match?(/rediss:\/\//)

        params[:ssl_params] = {verify_mode: OpenSSL::SSL::VERIFY_NONE}
      end
    end

    # Build HTTP health server parameters
    def to_http_health_params
      {
        port: http_health_port,
        path: http_health_path
      }
    end

    private

    def parse_sentinel(sentinel)
      return sentinel.transform_keys!(&:to_sym) if sentinel.is_a?(Hash)

      uri = URI.parse("redis://#{sentinel}")

      {host: uri.host, port: uri.port}.tap do |opts|
        opts[:password] = uri.password if uri.password
      end
    end
  end
end
