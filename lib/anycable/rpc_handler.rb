# frozen_string_literal: true

require "anycable/socket"
require "anycable/rpc/rpc_pb"
require "anycable/rpc/rpc_services_pb"

require "anycable/handler/capture_exceptions"

# rubocop:disable Metrics/AbcSize
# rubocop:disable Metrics/MethodLength
module Anycable
  # RPC service handler
  class RPCHandler < Anycable::RPC::Service
    prepend Anycable::Handler::CaptureExceptions

    # Handle connection request from WebSocket server
    def connect(request, _unused_call)
      logger.debug("RPC Connect: #{request.inspect}")

      socket = build_socket(env: rack_env(request))

      connection = factory.call(socket)

      connection.handle_open

      if socket.closed?
        Anycable::ConnectionResponse.new(status: Anycable::Status::FAILURE)
      else
        Anycable::ConnectionResponse.new(
          status: Anycable::Status::SUCCESS,
          identifiers: connection.identifiers_json,
          transmissions: socket.transmissions
        )
      end
    end

    def disconnect(request, _unused_call)
      logger.debug("RPC Disconnect: #{request.inspect}")

      socket = build_socket(env: rack_env(request))

      connection = factory.call(
        socket,
        identifiers: request.identifiers,
        subscriptions: request.subscriptions
      )

      if connection.handle_close
        Anycable::DisconnectResponse.new(status: Anycable::Status::SUCCESS)
      else
        Anycable::DisconnectResponse.new(status: Anycable::Status::FAILURE)
      end
    end

    def command(message, _unused_call)
      logger.debug("RPC Command: #{message.inspect}")

      socket = build_socket

      connection = factory.call(
        socket,
        identifiers: message.connection_identifiers
      )

      result = connection.handle_channel_command(
        message.identifier,
        message.command,
        message.data
      )

      Anycable::CommandResponse.new(
        status: result ? Anycable::Status::SUCCESS : Anycable::Status::FAILURE,
        disconnect: socket.closed?,
        stop_streams: socket.stop_streams?,
        streams: socket.streams,
        transmissions: socket.transmissions
      )
    end

    private

    # Build env from path
    def rack_env(request)
      uri = URI.parse(request.path)
      {
        "QUERY_STRING" => uri.query,
        "SCRIPT_NAME" => "",
        "PATH_INFO" => uri.path,
        "SERVER_PORT" => uri.port.to_s,
        "HTTP_HOST" => uri.host,
        # Hack to avoid Missing rack.input error
        "rack.request.form_input" => "",
        "rack.input" => "",
        "rack.request.form_hash" => {}
      }.merge(build_headers(request.headers))
    end

    def build_socket(**options)
      Anycable::Socket.new(options)
    end

    def build_headers(headers)
      headers.each_with_object({}) do |(k, v), obj|
        k = k.upcase
        k.tr!("-", "_")
        obj["HTTP_#{k}"] = v
      end
    end

    def logger
      Anycable.logger
    end

    def factory
      Anycable.connection_factory
    end
  end
end
# rubocop:enable Metrics/AbcSize
# rubocop:enable Metrics/MethodLength
