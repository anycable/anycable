# frozen_string_literal: true
require 'anycable/socket'
require 'anycable/rpc/rpc'
require 'anycable/rpc/rpc_services'

# rubocop:disable Metrics/AbcSize
# rubocop:disable Metrics/MethodLength
module Anycable
  # RPC service handler
  class RPCHandler < Anycable::RPC::Service
    # Handle connection request from WebSocket server
    def connect(request, _unused_call)
      logger.debug("RPC Connect: #{request}")

      socket = build_socket(env: rack_env(request))

      connection = factory.create(socket)

      connection.handle_open

      if socket.closed?
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR)
      else
        Anycable::ConnectionResponse.new(
          status: Anycable::Status::SUCCESS,
          identifiers: connection.identifiers_json,
          transmissions: socket.transmissions
        )
      end
    end

    def disconnect(request, _unused_call)
      logger.debug("RPC Disonnect: #{request}")

      socket = build_socket(env: rack_env(request))

      connection = factory.create(
        socket,
        identifiers: request.identifiers,
        subscriptions: request.subscriptions
      )

      if connection.handle_close
        Anycable::DisconnectResponse.new(status: Anycable::Status::SUCCESS)
      else
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR)
      end
    end

    def command(message, _unused_call)
      logger.debug("RPC Command: #{message}")

      socket = build_socket

      connection = factory.create(
        socket,
        identifiers: message.connection_identifiers
      )

      result = connection.handle_channel_command(
        message.identifier,
        message.command,
        message.data
      )

      Anycable::CommandResponse.new(
        status: result ? Anycable::Status::SUCCESS : Anycable::Status::ERROR,
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
        'QUERY_STRING' => uri.query,
        'SCRIPT_NAME' => '',
        'PATH_INFO' => uri.path,
        'SERVER_PORT' => uri.port.to_s,
        'HTTP_HOST' => uri.host,
        'HTTP_COOKIE' => request.headers['Cookie'],
        # Hack to avoid Missing rack.input error
        'rack.request.form_input' => '',
        'rack.input' => '',
        'rack.request.form_hash' => {}
      }
    end

    def build_socket(**options)
      Anycable::Socket.new(**options)
    end

    def logger
      Anycable.logger
    end

    def factory
      Anycable.config.connection_factory
    end
  end
end
