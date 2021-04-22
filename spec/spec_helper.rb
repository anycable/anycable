# frozen_string_literal: true

begin
  require "pry-byebug"
rescue LoadError
end

PROJECT_ROOT = File.expand_path("../", __dir__)

ENV["ANYCABLE_CONF"] = File.join(File.dirname(__FILE__), "support/anycable.yml")

require "anycable"

# Whether to run tests without GRPC loaded
NO_GRPC = !defined?(::GRPC)

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

  config.before do
    Anyway.env.clear if defined?(Anyway::Config)
  end

  config.after do
    AnyCable.logger.reset if AnyCable.logger.respond_to?(:reset)
    TestExHandler.flush!
  end
end
