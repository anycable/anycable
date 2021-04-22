# frozen_string_literal: true

require "anycable/socket"
require "anycable/rpc"
require "anycable/grpc/rpc_services_pb"

module AnyCable
  module GRPC
    # RPC service handler
    class Handler < AnyCable::GRPC::Service
      # Handle connection request from WebSocket server
      def connect(request, _unused_call)
        AnyCable.rpc_handler.handle(:connect, request)
      end

      def disconnect(request, _unused_call)
        AnyCable.rpc_handler.handle(:disconnect, request)
      end

      def command(request, _unused_call)
        AnyCable.rpc_handler.handle(:command, request)
      end
    end
  end
end
