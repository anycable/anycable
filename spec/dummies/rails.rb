# frozen_string_literal: true

# This path should be relative to <root>/bin, since we're copy app file there during tests
require_relative "../../spec/support/test_server"

module Rails
end

AnyCable.connection_factory = -> {}
AnyCable.server_builder ||= AnyCable::TestServer
