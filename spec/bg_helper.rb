# frozen_string_literal: true

Thread.new do
  Anycable.config.http_health_port = 54_321
  Anycable::Server.start
end
