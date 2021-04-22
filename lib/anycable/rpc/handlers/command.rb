# frozen_string_literal: true

module AnyCable
  module RPC
    module Handlers
      module Command
        def command(message)
          logger.debug("RPC Command: #{message.inspect}")

          socket = build_socket(env: rack_env(message.env))

          connection = factory.call(
            socket,
            identifiers: message.connection_identifiers
          )

          result = connection.handle_channel_command(
            message.identifier,
            message.command,
            message.data
          )

          AnyCable::CommandResponse.new(
            status: result ? AnyCable::Status::SUCCESS : AnyCable::Status::FAILURE,
            disconnect: socket.closed?,
            stop_streams: socket.stop_streams?,
            streams: socket.streams[:start],
            stopped_streams: socket.streams[:stop],
            transmissions: socket.transmissions,
            env: build_env_response(socket)
          )
        end
      end
    end
  end
end
