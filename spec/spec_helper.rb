# frozen_string_literal: true
ENV["RAILS_ENV"] = "test"

require "pry-byebug"

if ENV['COVER']
  require 'simplecov'
  SimpleCov.root File.join(File.dirname(__FILE__), '..')
  SimpleCov.add_filter "/spec/"
  SimpleCov.start
end

require File.expand_path("../dummy/config/environment", __FILE__)
require "anycable/server"

Rails.application.eager_load!

Dir["#{File.dirname(__FILE__)}/support/**/*.rb"].each { |f| require f }

RSpec.configure do |config|
  config.mock_with :rspec do |mocks|
    mocks.verify_partial_doubles = true
  end

  config.example_status_persistence_file_path = "tmp/rspec_examples.txt"
  config.filter_run :focus
  config.run_all_when_everything_filtered = true

  config.order = :random
  Kernel.srand config.seed

  config.after(:each) { ApplicationCable::Connection.events_log.clear }
end
