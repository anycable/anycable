# frozen_string_literal: true

require "anycable/socket"
require "anycable/rpc"
require "anycable/grpc/rpc_services_pb"

module AnyCable
  module GRPC
    # RPC service handler
    class Handler < AnyCable::GRPC::Service
      # Handle connection request from WebSocket server
      def connect(request, call)
        AnyCable.rpc_handler.handle(:connect, request, call.metadata)
      end

      def disconnect(request, call)
        AnyCable.rpc_handler.handle(:disconnect, request, call.metadata)
      end

      def command(request, call)
        AnyCable.rpc_handler.handle(:command, request, call.metadata)
      end
    end
  end
end
