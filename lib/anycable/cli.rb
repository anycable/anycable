# frozen_string_literal: true

require "optparse"

require "anycable"

$stdout.sync = true

module Anycable
  # Command-line interface for running AnyCable gRPC server
  # rubocop:disable Metrics/ClassLength
  class CLI
    # (not-so-big) List of common boot files for
    # different applications
    APP_CANDIDATES = %w[
      ./config/environment.rb
      ./config/anycable.rb
    ].freeze

    attr_reader :server, :health_server

    # rubocop:disable Metrics/AbcSize, Metrics/MethodLength
    def run(args)
      parse_options!(args)

      log_grpc! if config.log_grpc

      boot_app!

      @server = Anycable::Server.new(
        host: config.rpc_host,
        **config.to_grpc_params
      )

      start_health_server! if config.http_health_port_provided?
      start_pubsub!

      server.start
      server.wait_till_terminated
    end
    # rubocop:enable Metrics/AbcSize, Metrics/MethodLength

    private

    attr_reader :boot_file

    def config
      Anycable.config
    end

    def logger
      Anycable.logger
    end

    # rubocop:disable Metrics/AbcSize, Metrics/MethodLength
    def boot_app!
      @boot_file ||= try_detect_app

      if boot_file.nil?
        logger.fatal(
          "Couldn't find an application to load. " \
          "Please specify the explicit path via -r option, e.g:" \
          " anycable -r ./config/boot.rb or anycable -r /app/config/load_me.rb"
        )
        exit(1)
      end

      logger.info "Loading application from #{boot_file} ..."

      begin
        require boot_file
      rescue LoadError => e
        logger.fatal(
          "Failed to load application: #{e.message}. " \
          "Please specify the explicit path via -r option, e.g:" \
          " anycable -r ./config/boot.rb or anycable -r /app/config/load_me.rb"
        )
        exit(1)
      end

      if defined?(::Rails)
        logger.info "Rails application is loaded"
      else
        logger.info "Application is loaded"
      end
    end
    # rubocop:enable Metrics/AbcSize, Metrics/MethodLength

    def try_detect_app
      APP_CANDIDATES.detect { |path| File.exist?(path) }
    end

    def start_health_server!
      @health_server = Anycable::HealthServer.new(
        server,
        **config.to_http_health_params
      )
      health_server.start
    end

    def start_pubsub!
      logger.info "Broadcasting Redis channel: #{config.redis_channel}"
    end

    def log_grpc!
      ::GRPC.define_singleton_method(:logger) { Anycable.logger }
    end

    def parse_options!(args)
      unknown_opts = parse_cli_options!(args)

      begin
        config.parse_options!(unknown_opts)
      rescue OptionParser::InvalidOption => e
        $stdout.puts e.message
        $stdout.puts "Run anycable -h to see available options"
        exit(1)
      end
    end

    # rubocop:disable Metrics/MethodLength
    def parse_cli_options!(args)
      unknown_opts = []

      parser = build_cli_parser

      begin
        parser.parse!(args)
      rescue OptionParser::InvalidOption => e
        unknown_opts << e.args[0]
        unless args.size.zero?
          unknown_opts << args.shift unless args.first.start_with?("-")
          retry
        end
      end

      unknown_opts
    end

    def build_cli_parser
      OptionParser.new do |o|
        o.on "-v", "--version", "Print version and exit" do |_arg|
          $stdout.puts "AnyCable v#{Anycable::VERSION}"
          exit(0)
        end

        o.on "-r", "--require [PATH|DIR]", "Location of application file to require" do |arg|
          @boot_file = arg
        end

        o.on_tail "-h", "--help", "Show help" do
          $stdout.puts usage
          exit(0)
        end
      end
    end
    # rubocop:enable Metrics/MethodLength

    def usage
      <<~HELP
        anycable: run AnyCable gRPC server (https://anycable.io)

        VERSION
          anycable/#{Anycable::VERSION}

        USAGE
          $ anycable [options]

        BASIC OPTIONS
            -r, --require=path                Location of application file to require, default: "config/environment.rb"
            --rpc-host=host                   Local address to run gRPC server on, default: "[::]:50051"
            --redis-url=url                   Redis URL for pub/sub, default: REDIS_URL or "redis://localhost:6379/5"
            --redis-channel=name              Redis channel for broadcasting, default: "__anycable__"
            --log-level=level                 Logging level, default: "info"
            --log-file=path                   Path to log file, default: <none> (log to STDOUT)
            --log-grpc                        Enable gRPC logging (disabled by default)
            --debug                           Turn on verbose logging ("debug" level and gRPC logging on)
            -v, --version                     Print version and exit
            -h, --help                        Show this help

        HTTP HEALTH CHECKER OPTIONS
            --http-health-port=port           Port to run HTTP health server on, default: <none> (disabled)
            --http-health-path=path           Endpoint to server health cheks, default: "/health"

        GRPC OPTIONS
            --rpc-pool-size=size              gRPC workers pool size, default: 30
            --rpc-max-waiting-requests=num    Max waiting requests queue size, default: 20
            --rpc-poll-period=seconds         Poll period (sec), default: 1
            --rpc-pool-keep-alive=seconds     Keep-alive polling interval (sec), default: 1
      HELP
    end
  end
  # rubocop:enable Metrics/ClassLength
end
