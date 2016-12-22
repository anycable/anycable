require_relative "boot"
require "action_cable/engine"
require "global_id"

Bundler.require(*Rails.groups)
require "anycable"

module Dummy
  class Application < Rails::Application
    # Settings in config/environments/* take precedence over those specified here.
    # Application configuration should go into files in config/initializers
    # -- all .rb files in that directory are automatically loaded.

    # Only loads a smaller set of middleware suitable for API only apps.
    # Middleware like session, flash, cookies can be added back manually.
    # Skip views, helpers and assets when generating a new resource.
    config.api_only = true
    config.logger = Logger.new('/dev/null')
    config.eager_load = false
  end
end

