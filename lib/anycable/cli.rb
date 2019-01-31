# frozen_string_literal: true

require "optparse"

require "anycable"

$stdout.sync = true

module AnyCable
  # Command-line interface for running AnyCable gRPC server
  # rubocop:disable Metrics/ClassLength
  class CLI
    # (not-so-big) List of common boot files for
    # different applications
    APP_CANDIDATES = %w[
      ./config/anycable.rb
      ./config/environment.rb
    ].freeze

    # Wait for external process termination (s)
    WAIT_PROCESS = 2

    attr_reader :server, :health_server

    # rubocop:disable Metrics/AbcSize, Metrics/MethodLength
    def run(args = {})
      @at_stop = []

      extra_options = parse_cli_options!(args)

      # Boot app first, 'cause it might change
      # configuration, loggin settings, etc.
      boot_app!

      parse_gem_options!(extra_options)

      configure_server!

      logger.info "Starting AnyCable gRPC server (pid: #{Process.pid})"

      print_versions!

      logger.info "Serving #{defined?(::Rails) ? 'Rails ' : ''}application from #{boot_file}"

      verify_connection_factory!

      log_grpc! if config.log_grpc

      log_errors!

      @server = AnyCable::Server.new(
        host: config.rpc_host,
        **config.to_grpc_params,
        interceptors: AnyCable.middleware.to_a
      )

      # Make sure middlewares are not adding after server has started
      AnyCable.middleware.freeze

      start_health_server! if config.http_health_port_provided?
      start_pubsub!

      server.start

      run_custom_server_command! unless server_command.nil?

      begin
        wait_till_terminated
      rescue Interrupt => e
        logger.info "Stopping... #{e.message}"

        shutdown

        logger.info "Stopped. Good-bye!"
        exit(0)
      end
    end
    # rubocop:enable Metrics/AbcSize, Metrics/MethodLength

    def shutdown
      at_stop.each(&:call)
      server.stop
    end

    private

    attr_reader :boot_file, :server_command

    def config
      AnyCable.config
    end

    def logger
      AnyCable.logger
    end

    def at_stop
      if block_given?
        @at_stop << Proc.new
      else
        @at_stop
      end
    end

    def wait_till_terminated
      self_read = setup_signals

      while readable_io = IO.select([self_read]) # rubocop:disable Lint/AssignmentInCondition
        signal = readable_io.first[0].gets.strip
        raise Interrupt, "SIG#{signal} received"
      end
    end

    def setup_signals
      self_read, self_write = IO.pipe

      %w[INT TERM].each do |signal|
        trap signal do
          self_write.puts signal
        end
      end

      self_read
    end

    def print_versions!
      logger.info "AnyCable version: #{AnyCable::VERSION}"
      logger.info "gRPC version: #{GRPC::VERSION}"
    end

    # rubocop:disable Metrics/MethodLength
    def boot_app!
      @boot_file ||= try_detect_app

      if boot_file.nil?
        $stdout.puts(
          "Couldn't find an application to load. " \
          "Please specify the explicit path via -r option, e.g:" \
          " anycable -r ./config/boot.rb or anycable -r /app/config/load_me.rb"
        )
        exit(1)
      end

      begin
        require boot_file
      rescue LoadError => e
        $stdout.puts(
          "Failed to load application: #{e.message}. " \
          "Please specify the explicit path via -r option, e.g:" \
          " anycable -r ./config/boot.rb or anycable -r /app/config/load_me.rb"
        )
        exit(1)
      end
    end
    # rubocop:enable Metrics/MethodLength

    def try_detect_app
      APP_CANDIDATES.detect { |path| File.exist?(path) }
    end

    def configure_server!
      AnyCable.server_callbacks.each(&:call)
    end

    def start_health_server!
      @health_server = AnyCable::HealthServer.new(
        server,
        **config.to_http_health_params
      )
      health_server.start

      at_stop { health_server.stop }
    end

    def start_pubsub!
      logger.info "Broadcasting Redis channel: #{config.redis_channel}"
    end

    # rubocop: disable Metrics/MethodLength, Metrics/AbcSize
    def run_custom_server_command!
      pid = nil
      stopped = false
      command_thread = Thread.new do
        pid = Process.spawn(server_command)
        logger.info "Started command: #{server_command} (pid: #{pid})"

        Process.wait pid
        pid = nil
        raise Interrupt, "Server command exit unexpectedly" unless stopped
      end

      command_thread.abort_on_exception = true

      at_stop do
        stopped = true
        next if pid.nil?

        Process.kill("SIGTERM", pid)

        logger.info "Wait till process #{pid} stop..."

        tick = 0

        loop do
          tick += 0.2
          break if tick > WAIT_PROCESS

          if pid.nil?
            logger.info "Process #{pid} stopped."
            break
          end
        end
      end
    end
    # rubocop: enable Metrics/MethodLength, Metrics/AbcSize

    def log_grpc!
      ::GRPC.define_singleton_method(:logger) { AnyCable.logger }
    end

    # Add default exceptions handler: print error message to log
    def log_errors!
      if AnyCable.config.debug?
        # Print error with backtrace in debug mode
        AnyCable.capture_exception do |e|
          AnyCable.logger.error("#{e.message}:\n#{e.backtrace.take(20).join("\n")}")
        end
      else
        AnyCable.capture_exception { |e| AnyCable.logger.error(e.message) }
      end
    end

    def verify_connection_factory!
      return if AnyCable.connection_factory

      logger.error "AnyCable connection factory must be configured. " \
                   "Make sure you've required a gem (e.g. `anycable-rails`) or " \
                   "configured `AnyCable.connection_factory` yourself"
      exit(1)
    end

    def parse_gem_options!(args)
      config.parse_options!(args)
    rescue OptionParser::InvalidOption => e
      $stdout.puts e.message
      $stdout.puts "Run anycable -h to see available options"
      exit(1)
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
          $stdout.puts "AnyCable v#{AnyCable::VERSION}"
          exit(0)
        end

        o.on "-r", "--require [PATH|DIR]", "Location of application file to require" do |arg|
          @boot_file = arg
        end

        o.on "--server-command VALUE", "Command to run WebSocket server" do |arg|
          @server_command = arg
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
          anycable/#{AnyCable::VERSION}

        USAGE
          $ anycable [options]

        BASIC OPTIONS
            -r, --require=path                Location of application file to require, default: "config/environment.rb"
            --server-command=command          Command to run WebSocket server
            --rpc-host=host                   Local address to run gRPC server on, default: "[::]:50051"
            --redis-url=url                   Redis URL for pub/sub, default: REDIS_URL or "redis://localhost:6379/5"
            --redis-channel=name              Redis channel for broadcasting, default: "__anycable__"
            --redis-sentinels=<...hosts>      Redis Sentinel followers addresses (as a comma-separated list), default: nil
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
