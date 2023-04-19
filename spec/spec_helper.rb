# frozen_string_literal: true

begin
  require "debug" unless ENV["CI"] == "true"
rescue LoadError
end

if ENV["COVERAGE"] == "true"
  require "simplecov"
  SimpleCov.start do
    enable_coverage :branch

    add_filter "/spec/"
    add_filter "/lib/anycable/protos/"
    add_filter "/lib/anycable/grpc_kit/health_*.rb"
  end

  require "simplecov-lcov"
  SimpleCov::Formatter::LcovFormatter.config do |c|
    c.report_with_single_file = true
    c.single_report_path = "coverage/lcov.info"
  end

  SimpleCov.formatters = SimpleCov::Formatter::MultiFormatter.new([
    SimpleCov::Formatter::HTMLFormatter,
    SimpleCov::Formatter::LcovFormatter
  ])
end

PROJECT_ROOT = File.expand_path("../", __dir__)

ENV["ANYCABLE_CONF"] = File.join(File.dirname(__FILE__), "support/anycable.yml")

require "anycable"

# Whether to run tests without GRPC loaded
NO_GRPC = !defined?(::GRPC)
GRPC_KIT = ENV["ANYCABLE_GRPC_IMPL"] == "grpc_kit"

if GRPC_KIT
  $stdout.puts "⚠️ Testing against grpc_kit"
end

require "json"
require "rack"

require "anycable/rspec"

require "webmock/rspec"
WebMock.disable_net_connect!(allow_localhost: true)

Dir["#{File.dirname(__FILE__)}/support/**/*.rb"].sort.each { |f| require f }

AnyCable.connection_factory = AnyCable::TestFactory
AnyCable.logger = TestLogger.new

if ENV["LOG"]
  AnyCable.logger = Logger.new($stdout)
  ::GRPC.define_singleton_method(:logger) { AnyCable.logger } if defined?(::GRPC)
end

module TestExHandler
  Error = Struct.new(:exception, :method, :message)

  class << self
    attr_reader :last_error

    def call(exp, method, message)
      @last_error = Error.new(exp, method, message)
    end

    def flush!
      @last_error = nil
    end
  end
end

AnyCable.capture_exception(&TestExHandler.method(:call))

# Make sure that tmp is here (for CI)
FileUtils.mkdir_p("tmp")

RSpec.configure do |config|
  config.mock_with :rspec do |mocks|
    mocks.verify_partial_doubles = true
  end

  config.include WithEnv

  config.example_status_persistence_file_path = "tmp/rspec_examples.txt"
  config.filter_run :focus
  config.run_all_when_everything_filtered = true

  config.order = :random
  Kernel.srand config.seed

  config.define_derived_metadata(file_path: %r{/grpc/}) do |metadata|
    metadata[:grpc] = true
  end
  config.filter_run_excluding(grpc: true) if NO_GRPC
  # Igonore specs manually checking for argument types when running RBS runtime tester
  config.filter_run_excluding(rbs: false) if defined?(::RBS::Test)

  config.before do
    Anyway.env.clear if defined?(Anyway::Config)
  end

  config.after do
    AnyCable.logger.reset if AnyCable.logger.respond_to?(:reset)
    TestExHandler.flush!
  end
end
