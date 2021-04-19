# frozen_string_literal: true

module AnyCable
  module RPC
    module Handlers
      module Connect
        using Helpers

        module_function

        def call(request)
          logger.debug("RPC Connect: #{request.inspect}")

          socket = build_socket(env: rack_env(request.env))

          connection = factory.call(socket)

          connection.handle_open

          if socket.closed?
            AnyCable::ConnectionResponse.new(
              status: AnyCable::Status::FAILURE,
              transmissions: socket.transmissions
            )
          else
            AnyCable::ConnectionResponse.new(
              status: AnyCable::Status::SUCCESS,
              identifiers: connection.identifiers_json,
              transmissions: socket.transmissions,
              env: build_env_response(socket)
            )
          end
        end
      end
    end
  end
end
