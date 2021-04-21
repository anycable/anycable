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
        AnyCable::RPC::Handlers::Connect.call(request)
      rescue => exp
        notify_exception(exp, :connect, request)

        AnyCable::ConnectionResponse.new(
          status: AnyCable::Status::ERROR,
          error_msg: exp.message
        )
      end

      def disconnect(request, _unused_call)
        AnyCable::RPC::Handlers::Disconnect.call(request)
      rescue => exp
        notify_exception(exp, :disconnect, request)

        AnyCable::DisconnectResponse.new(
          status: AnyCable::Status::ERROR,
          error_msg: exp.message
        )
      end

      def command(request, _unused_call)
        AnyCable::RPC::Handlers::Command.call(request)
      rescue => exp
        notify_exception(exp, :command, request)

        AnyCable::CommandResponse.new(
          status: AnyCable::Status::ERROR,
          error_msg: exp.message
        )
      end

      private

      def notify_exception(exp, method_name, message)
        AnyCable::ExceptionsHandling.notify(exp, method_name.to_s, message.to_h)
      end
    end
  end
end
