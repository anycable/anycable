# frozen_string_literal: true

module AnyCable
  module RPC
    module Handlers
      module Disconnect
        def disconnect(request)
          logger.debug("RPC Disconnect: #{request.inspect}")

          socket = build_socket(env: rack_env(request.env))

          connection = factory.call(
            socket,
            identifiers: request.identifiers,
            subscriptions: request.subscriptions
          )

          if connection.handle_close
            AnyCable::DisconnectResponse.new(status: AnyCable::Status::SUCCESS)
          else
            AnyCable::DisconnectResponse.new(status: AnyCable::Status::FAILURE)
          end
        end
      end
    end
  end
end
