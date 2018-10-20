# frozen_string_literal: true

require "optparse"
require "pry-byebug"

require "anycable"

module Anycable
  # Command-line interface for running AnyCable gRPC server
  class CLI
    def run(args)
      unknown_opts = parse_cli_options!(args)

      begin
        Anycable.config.parse_options!(unknown_opts)
      rescue OptionParser::InvalidOption => e
        $stdout.puts e.message
        # TODO: print usage
        exit(1)
      end

      Anycable::Server.start
    end

    private

    attr_reader :boot_file

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
    # rubocop:enable Metrics/MethodLength

    def build_cli_parser
      OptionParser.new do |o|
        o.on "-v", "--version", "Print version and exit" do |_arg|
          $stdout.puts "AnyCable v#{Anycable::VERSION}"
          exit(0)
        end

        o.on "-r", "--require [PATH|DIR]", "Location of application file to require" do |arg|
          @boot_file = arg
        end
      end
    end
  end
end
