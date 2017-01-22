# frozen_string_literal: true
require 'anycable/rpc/rpc'
require 'anycable/rpc/rpc_services'

# rubocop:disable Metrics/ClassLength
# rubocop:disable Metrics/MethodLength
module Anycable
  # RPC service handler
  class RPCHandler < Anycable::RPC::Service
    # Handle connection request from WebSocket server
    def connect(request, _unused_call)
      logger.debug("RPC Connect: #{request}")

      connection = factory.create(env: rack_env(request))

      connection.handle_open

      if connection.closed?
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR)
      else
        Anycable::ConnectionResponse.new(
          status: Anycable::Status::SUCCESS,
          identifiers: connection.identifiers_json,
          transmissions: connection.transmissions
        )
      end
    end

    def disconnect(request, _unused_call)
      logger.debug("RPC Disonnect: #{request}")

      connection = factory.create(
        env: rack_env(request),
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

      run_command(message)
    end

    private

    def run_command(message)
      connection = factory.create(
        identifiers: message.connection_identifiers
      )

      channel = connection.channel_for(message.identifier)

      if !channel.nil?
        send("handle_#{message.command}", message, connection, channel)
      else
        Anycable::CommandResponse.new(
          status: Anycable::Status::ERROR
        )
      end
    end

    def handle_subscribe(_msg, connection, channel)
      channel.handle_subscribe
      if channel.subscription_rejected?
        Anycable::CommandResponse.new(
          status: Anycable::Status::ERROR,
          disconnect: connection.closed?,
          transmissions: connection.transmissions
        )
      else
        Anycable::CommandResponse.new(
          status: Anycable::Status::SUCCESS,
          disconnect: connection.closed?,
          stop_streams: channel.stop_streams?,
          streams: channel.streams,
          transmissions: connection.transmissions
        )
      end
    end

    def handle_unsubscribe(_mgs, connection, channel)
      channel.handle_unsubscribe

      Anycable::CommandResponse.new(
        status: Anycable::Status::SUCCESS,
        disconnect: connection.closed?,
        stop_streams: true,
        transmissions: connection.transmissions
      )
    end

    def handle_message(msg, connection, channel)
      channel.handle_action(msg.data)
      Anycable::CommandResponse.new(
        status: Anycable::Status::SUCCESS,
        disconnect: connection.closed?,
        stop_streams: channel.stop_streams?,
        streams: channel.streams,
        transmissions: connection.transmissions
      )
    end

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

    def logger
      Anycable.logger
    end

    def factory
      Anycable.config.connection_factory
    end
  end
end
