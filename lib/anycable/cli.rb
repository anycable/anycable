# frozen_string_literal: true

require "optparse"

require "anycable"

$stdout.sync = true

module AnyCable
  # Command-line interface for running AnyCable RPC server
  class CLI
    # (not-so-big) List of common boot files for
    # different applications
    APP_CANDIDATES = %w[
      ./config/anycable.rb
      ./config/environment.rb
    ].freeze

    # Wait for external process termination (s)
    WAIT_PROCESS = 2

    # Run CLI inside the current process and shutdown at exit
    def self.embed!(args = [])
      new(embedded: true).tap do |cli|
        cli.run(args)
        at_exit { cli.shutdown }
      end
    end

    attr_reader :server, :health_server, :embedded
    alias_method :embedded?, :embedded

    def initialize(embedded: false)
      @embedded = embedded
    end

    def run(args = [])
      @at_stop = []

      extra_options = parse_cli_options!(args)

      # Boot app first, 'cause it might change
      # configuration, loggin settings, etc.
      boot_app! unless embedded?

      # Make sure Rails extensions for Anyway Config are loaded
      # See https://github.com/anycable/anycable-rails/issues/63
      require "anyway/rails" if defined?(::Rails::VERSION)

      parse_gem_options!(extra_options)

      configure_server!

      logger.info "Starting AnyCable RPC server (pid: #{Process.pid})"

      print_version!

      logger.info "Serving #{defined?(::Rails) ? "Rails " : ""}application from #{boot_file}" unless embedded?

      verify_connection_factory!

      log_errors!

      verify_server_builder!

      @server = AnyCable.server_builder.call(config)

      # Make sure middlewares are not adding after server has started
      AnyCable.middleware.freeze

      start_health_server! if config.http_health_port_provided?
      start_pubsub!

      server.start

      run_custom_server_command! unless server_command.nil?

      return if embedded?

      begin
        wait_till_terminated
      rescue Interrupt => e
        logger.info "Stopping... #{e.message}"

        shutdown

        logger.info "Stopped. Good-bye!"
        exit(0)
      end
    end

    def shutdown
      at_stop.each(&:call)
      server&.stop
    end

    private

    attr_reader :boot_file, :server_command

    def config
      AnyCable.config
    end

    def logger
      AnyCable.logger
    end

    def at_stop(&block)
      if block
        @at_stop << block
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

    def print_version!
      logger.info "AnyCable version: #{AnyCable::VERSION} (proto_version: #{AnyCable::PROTO_VERSION})"
    end

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
      AnyCable.broadcast_adapter.announce!
    end

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

        tick = 0.0

        loop do
          tick += 0.2
          # @type break: nil
          break if tick > WAIT_PROCESS

          if pid.nil?
            logger.info "Process #{pid} stopped."
            # @type break: nil
            break
          end
        end
      end
    end

    # Add default exceptions handler: print error message to log
    def log_errors!
      if AnyCable.config.debug?
        # Print error with backtrace in debug mode
        AnyCable.capture_exception do |e|
          stack = e.backtrace
          backtrace = stack ? ":\n#{stack.take(20).join("\n")}" : ""
          AnyCable.logger.error("#{e.message}#{backtrace}")
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

    def verify_server_builder!
      return if AnyCable.server_builder

      logger.error "AnyCable server builder must be configured. " \
                   "Make sure you've required a gem (e.g. `anycable-grpc`) or " \
                   "configured `AnyCable.server_builder` yourself"
      exit(1)
    end

    def parse_gem_options!(args)
      config.parse_options!(args)
    rescue OptionParser::InvalidOption => e
      $stdout.puts e.message
      $stdout.puts "Run anycable -h to see available options"
      exit(1)
    end

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

    def usage
      usage_header =
        <<~HELP
          anycable: run AnyCable RPC server (https://anycable.io)

          VERSION
            anycable/#{AnyCable::VERSION}

          USAGE
            $ anycable [options]

          CLI
              -r, --require=path                Location of application file to require, default: "config/environment.rb"
              --server-command=command          Command to run WebSocket server
              -v, --version                     Print version and exit
              -h, --help                        Show this help
        HELP

      [usage_header, *AnyCable::Config.usages].join("\n")
    end
  end
end
