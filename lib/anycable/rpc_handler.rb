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
      # TODO: implement disconnect logic
      Anycable::DisconnectResponse.new(status: Anycable::Status::SUCCESS)
    end

    def subscribe(message, _unused_call)
      logger.debug("RPC Subscribe: #{message}")
      connection = ApplicationCable::Connection.new(
        identifiers_json: message.connection_identifiers
      )

      channel = channel_for(connection, message)

      if channel.present?
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
            stream_from: channel.streams.present?,
            stream_id: channel.streams.first || '',
            transmissions: connection.transmissions
          )
        end
      else
        Anycable::CommandResponse.new(
          status: Anycable::Status::ERROR
        )
      end
    end

    def unsubscribe(message, _unused_call)
      logger.debug("RPC Unsubscribe: #{message}")
      Anycable::CommandResponse.new(
        status: Anycable::Status::SUCCESS,
        disconnect: false,
        stop_streams: true,
        stream_from: false
      )
    end

    def perform(message, _unused_call)
      logger.debug("RPC Perform: #{message}")
      connection = ApplicationCable::Connection.new(
        identifiers_json: message.connection_identifiers
      )

      channel = channel_for(connection, message)

      if channel.present?
        channel.perform_action(ActiveSupport::JSON.decode(message.data))
        Anycable::CommandResponse.new(
          status: Anycable::Status::SUCCESS,
          disconnect: connection.closed?,
          stop_streams: channel.stop_streams?,
          stream_from: channel.streams.present?,
          stream_id: channel.streams.first || '',
          transmissions: connection.transmissions
        )
      else
        Anycable::CommandResponse.new(
          status: Anycable::Status::ERROR
        )
      end
    end

    private

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

    def channel_for(connection, message)
      id_key = message.identifier
      id_options = ActiveSupport::JSON.decode(id_key).with_indifferent_access

      subscription_klass = id_options[:channel].safe_constantize

      if subscription_klass
        subscription_klass.new(connection, id_key, id_options)
      else
        logger.error "Subscription class not found (#{message.inspect})"
        nil
      end
    end

    def logger
      Anycable.logger
    end
  end
end
