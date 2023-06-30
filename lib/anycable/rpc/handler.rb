# frozen_string_literal: true

require "anycable/rpc/handlers/connect"
require "anycable/rpc/handlers/disconnect"
require "anycable/rpc/handlers/command"

module AnyCable
  module RPC
    # Generic RPC handler
    class Handler
      include Handlers::Connect
      include Handlers::Disconnect
      include Handlers::Command

      def initialize(middleware: AnyCable.middleware)
        @middleware = middleware
        @commands = {}
      end

      def handle(cmd, data, meta = {})
        middleware.call(cmd, data, meta) do
          send(cmd, data)
        end
      end

      def supported?(cmd)
        %i[connect disconnect command].include?(cmd)
      end

      private

      attr_reader :commands, :middleware

      def build_socket(env:)
        AnyCable::Socket.new(env: env)
      end

      def build_env_response(socket)
        AnyCable::EnvResponse.new(
          cstate: socket.cstate.changed_fields,
          istate: socket.istate.changed_fields
        )
      end

      def logger
        AnyCable.logger
      end

      def factory
        AnyCable.connection_factory
      end
    end
  end
end
