# frozen_string_literal: true
require 'anycable/actioncable/connection'
require 'anycable/actioncable/channel'
require 'anycable/rpc/rpc'
require 'anycable/rpc/rpc_services'

# rubocop:disable Metrics/ClassLength
# rubocop:disable Metrics/AbcSize
# rubocop:disable Metrics/MethodLength
module Anycable
  # RPC service handler
  class RPCHandler < Anycable::RPC::Service
    # Handle connection request from WebSocket server
    def connect(request, _unused_call)
      logger.debug("RPC Connect: #{request}")

      connection = ApplicationCable::Connection.new(
        env:
          path_env(request.path).merge(
            'HTTP_COOKIE' => request.headers['Cookie']
          )
      )

      connection.handle_open

      if connection.closed?
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR)
      else
        Anycable::ConnectionResponse.new(
          status: Anycable::Status::SUCCESS,
          identifiers: connection.identifiers_hash.to_json,
          transmissions: connection.transmissions
        )
      end
    end

    def disconnect(request, _unused_call)
      logger.debug("RPC Disonnect: #{request}")

      connection = ApplicationCable::Connection.new(
        env:
          path_env(request.path).merge(
            'HTTP_COOKIE' => request.headers['Cookie']
          ),
        identifiers_json: request.identifiers,
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
      connection = ApplicationCable::Connection.new(
        identifiers_json: message.connection_identifiers
      )

      channel = connection.channel_for(message.identifier)

      if channel.present?
        send("handle_#{message.command}", message, connection, channel)
      else
        Anycable::CommandResponse.new(
          status: Anycable::Status::ERROR
        )
      end
    end

    def handle_subscribe(_msg, connection, channel)
      channel.do_subscribe
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
      connection.subscriptions.remove_subscription(channel)

      Anycable::CommandResponse.new(
        status: Anycable::Status::SUCCESS,
        disconnect: false,
        stop_streams: true
      )
    end

    def handle_message(msg, connection, channel)
      channel.perform_action(ActiveSupport::JSON.decode(msg.data))
      Anycable::CommandResponse.new(
        status: Anycable::Status::SUCCESS,
        disconnect: connection.closed?,
        stop_streams: channel.stop_streams?,
        streams: channel.streams,
        transmissions: connection.transmissions
      )
    end

    # Build env from path
    def path_env(path)
      uri = URI.parse(path)
      {
        'QUERY_STRING' => uri.query,
        'SCRIPT_NAME' => '',
        'PATH_INFO' => uri.path,
        'SERVER_PORT' => uri.port.to_s,
        'HTTP_HOST' => uri.host,
        # Hack to avoid Missing rack.input error
        'rack.request.form_input' => '',
        'rack.input' => '',
        'rack.request.form_hash' => {}
      }
    end

    def logger
      Anycable.logger
    end
  end
end
