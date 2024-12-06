# frozen_string_literal: true

module AnyCable
  module WSRPC
    class Server
      private attr_reader :logger

      def initialize(url: AnyCable.config.ws_rpc_url, token: AnyCable.config.http_rpc_secret, logger: AnyCable.logger)
        @url = url
        @token = token
        @logger = logger

        uri = URI.parse(url)

        if @token && !@token.empty?
          params = URI.decode_www_form(uri.query || "")
          params << ["token", @token]
          uri.query = URI.encode_www_form(params)
        end

        @client_url = uri.to_s
      end

      def build_client(app = nil, &block)
        app ||= block if block_given?
        app ||= self

        Client.new(app, @client_url)
      end

      def call(client, msg)
        event = JSON.parse(msg)
        type = event["type"]

        return logger.info("WebSocket RPC client connected") if type == "connect"

        if type == "disconnect"
          logger.info("WebSocket RPC client disconnected: #{event["reason"]}")
          event["reconnect"] ? client.open : client.close
          return
        end

        raise "Unknown RPC event" if type != "command"

        rpc_command = event["command"]
        rpc_payload = event["payload"]

        payload =
          case rpc_command
          when "connect" then AnyCable::ConnectionRequest.decode_json(rpc_payload)
          when "disconnect" then AnyCable::DisconnectRequest.decode_json(rpc_payload)
          when "command" then AnyCable::CommandMessage.decode_json(rpc_payload)
          end

        return logger.error("unknown RPC command: #{rpc_command}") if payload.nil?

        meta = event["meta"] || {}
        call_id = event.fetch("call_id") { raise "Missing call_id" }

        result = AnyCable.rpc_handler.handle(rpc_command.to_sym, payload, meta)

        response = {
          payload: result.to_json({format_enums_as_integers: true, preserve_proto_fieldnames: true}),
          call_id: call_id
        }.to_json

        client.send_message(response)
      end
    end
  end
end
