# frozen_string_literal: true

require "pry-byebug"

if ENV["COVER"]
  require "simplecov"
  SimpleCov.root File.join(File.dirname(__FILE__), "..")
  SimpleCov.add_filter "/spec/"
  SimpleCov.start
end

PROJECT_ROOT = File.expand_path("../", __dir__)

ENV["ANYCABLE_CONF"] = File.join(File.dirname(__FILE__), "support/anycable.yml")

require "anycable"
require "json"
require "rack"

Dir["#{File.dirname(__FILE__)}/support/**/*.rb"].each { |f| require f }

AnyCable.connection_factory = AnyCable::TestFactory
AnyCable.logger = TestLogger.new

AnyCable::Server.log_grpc! if ENV["LOG"]

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

  config.after(:each) do
    Anyway.env.clear if defined?(Anyway)
    AnyCable.logger.reset if AnyCable.logger.respond_to?(:reset)
    TestExHandler.flush!
  end
end
