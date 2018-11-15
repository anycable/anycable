# frozen_string_literal: true

# no-op file to pass as application code

AnyCable.connection_factory = -> {}

# print something when server is running

AnyCable.configure_server do
  p "Hello from app, server!"
end
