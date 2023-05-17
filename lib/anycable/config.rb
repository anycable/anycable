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
      presets: "",

      ## PubSub
      broadcast_adapter: :redis,

      ### Redis options
      redis_url: ENV.fetch("REDIS_URL", "redis://localhost:6379"),
      redis_sentinels: nil,
      redis_channel: "__anycable__",
      redis_tls_verify: false,
      redis_tls_client_cert_path: nil,
      redis_tls_client_key_path: nil,

      ### NATS options
      nats_servers: "nats://localhost:4222",
      nats_channel: "__anycable__",
      nats_dont_randomize_servers: false,
      nats_options: {},

      ### HTTP broadcasting options
      http_broadcast_url: "http://localhost:8090/_broadcast",
      http_broadcast_secret: nil,

      ### Logging options
      log_file: nil,
      log_level: "info",
      debug: false, # Shortcut to enable debug level and verbose logging

      ### Health check options
      http_health_port: nil,
      http_health_path: "/health",

      ### Misc options
      version_check_enabled: true,
      sid_header_enabled: true
    )

    coerce_types(
      presets: {type: nil, array: true},
      redis_sentinels: {type: nil, array: true},
      nats_servers: {type: nil, array: true},
      redis_tls_verify: :boolean,
      nats_dont_randomize_servers: :boolean,
      debug: :boolean,
      version_check_enabled: :boolean
    )

    flag_options :debug, :nats_dont_randomize_servers
    ignore_options :nats_options

    def load(*_args)
      super.tap { load_presets }
    end

    def log_level
      debug? ? "debug" : super
    end

    def http_health_port_provided?
      !http_health_port.nil? && http_health_port != ""
    end

    usage <<~TXT
      APPLICATION
          --broadcast-adapter=type          Pub/sub adapter type for broadcasts, default: redis
          --log-level=level                 Logging level, default: "info"
          --log-file=path                   Path to log file, default: <none> (log to STDOUT)
          --debug                           Turn on verbose logging ("debug" level and verbose logging on)

      HTTP HEALTH CHECKER
          --http-health-port=port           Port to run HTTP health server on, default: <none> (disabled)
          --http-health-path=path           Endpoint to serve health checks, default: "/health"

      REDIS PUB/SUB
          --redis-url=url                   Redis URL for pub/sub, default: REDIS_URL or "redis://localhost:6379/5"
          --redis-channel=name              Redis channel for broadcasting, default: "__anycable__"
          --redis-sentinels=<...hosts>      Redis Sentinel followers addresses (as a comma-separated list), default: nil
          --redis-tls-verify=yes|no         Whether to perform server certificate check in case of rediss:// protocol. Default: yes
          --redis-tls-client_cert-path=path Default: nil
          --redis-tls-client_key-path=path  Default: nil

      NATS PUB/SUB
          --nats-servers=<...addresses>     NATS servers for pub/sub, default: "nats://localhost:4222"
          --nats-channel=name               NATS channel for broadcasting, default: "__anycable__"
          --nats-dont-randomize-servers     Pass this option to disable NATS servers randomization during (re-)connect

      HTTP PUB/SUB
          --http-broadcast-url              HTTP pub/sub endpoint URL, default: "http://localhost:8090/_broadcast"
          --http-broadcast-secret           HTTP pub/sub authorization secret, default: <none> (disabled)
    TXT

    # Build Redis parameters
    def to_redis_params
      # @type var base_params: { url: String, sentinels: Array[untyped]?, ssl_params: Hash[Symbol, untyped]? }
      base_params = {url: redis_url}
      base_params.tap do |params|
        sentinels = redis_sentinels
        next if sentinels.nil? || sentinels.empty?

        sentinels = Array(sentinels) unless sentinels.is_a?(Array)

        next if sentinels.empty?

        params[:sentinels] = sentinels.map { |sentinel| parse_sentinel(sentinel) }
      end.tap do |params|
        next unless redis_url.match?(/rediss:\/\//)

        if !!redis_tls_client_cert_path ^ !!redis_tls_client_key_path
          raise_validation_error "Both Redis TLS client certificate and private key must be specified (or none of them)"
        end

        if !redis_tls_verify?
          params[:ssl_params] = {verify_mode: OpenSSL::SSL::VERIFY_NONE}
        else
          cert_path, key_path = redis_tls_client_cert_path, redis_tls_client_key_path
          if cert_path && key_path
            params[:ssl_params] = {
              cert: OpenSSL::X509::Certificate.new(File.read(cert_path)),
              key: OpenSSL::PKey.read(File.read(key_path))
            }
          end
        end
      end
    end

    # Build options for NATS.connect
    def to_nats_params
      {
        servers: Array(nats_servers),
        dont_randomize_servers: nats_dont_randomize_servers
      }.merge(nats_options)
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

    def load_presets
      if presets.nil? || presets.empty?
        self.presets = detect_presets
        __trace__&.record_value(presets, :presets, type: :env)
      end

      return if presets.empty?

      presets.each { send(:"load_#{_1}_presets") if respond_to?(:"load_#{_1}_presets", true) }
    end

    def detect_presets
      [].tap do
        _1 << "fly" if ENV.key?("FLY_APP_NAME") && ENV.key?("FLY_ALLOC_ID") && ENV.key?("FLY_REGION")
      end
    end

    def load_fly_presets
      write_preset(:rpc_host, "0.0.0.0:50051", preset: "fly")

      ws_app_name = ENV["ANYCABLE_FLY_WS_APP_NAME"]
      return unless ws_app_name

      region = ENV.fetch("FLY_REGION")

      write_preset(:http_broadcast_url, "http://#{region}.#{ws_app_name}.internal:8090/_broadcast", preset: "fly")
      write_preset(:nats_servers, "nats://#{region}.#{ws_app_name}.internal:4222", preset: "fly")
    end

    def write_preset(key, value, preset:)
      # do not override explicitly provided values
      return unless __trace__&.dig(key.to_s)&.source&.dig(:type) == :defaults

      write_config_attr(key, value)
      __trace__&.record_value(value, key, type: :preset, preset: preset)
    end
  end
end
