# frozen_string_literal: true

require_relative "../support/test_server"

# no-op file to pass as application code

AnyCable.connection_factory = -> {}
AnyCable.server_builder ||= AnyCable::TestServer

# print something when server is running

AnyCable.configure_server do
  p "Hello from app, server!"
end
