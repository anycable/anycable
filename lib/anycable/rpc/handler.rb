# frozen_string_literal: true

require "anycable/rpc/helpers"

require "anycable/rpc/handlers/connect"
require "anycable/rpc/handlers/disconnect"
require "anycable/rpc/handlers/command"

module AnyCable
  module RPC
    # Generic RPC handler
    class Handler
      include Helpers
      include Handlers::Connect
      include Handlers::Disconnect
      include Handlers::Command

      def initialize(middleware: AnyCable.middleware)
        @middleware = middleware
        @commands = {}
      end

      def handle(cmd, data)
        middleware.call(cmd, data) do
          send(cmd, data)
        end
      end

      private

      attr_reader :commands, :middleware
    end
  end
end
