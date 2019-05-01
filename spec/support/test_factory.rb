# frozen_string_literal: true

module AnyCable
  module TestFactory
    class Connection
      attr_reader :request, :socket, :identifiers, :subscriptions

      def initialize(socket, identifiers: nil, subscriptions: nil)
        @socket = socket
        @identifiers = identifiers ? JSON.parse(identifiers) : {}
        @request = Rack::Request.new(socket.env)
        @subscriptions = subscriptions
      end

      def handle_open
        raise request.params["raise"] if request.params["raise"]

        @identifiers["current_user"] = request.cookies["username"]
        @identifiers["path"] = request.path
        @identifiers["token"] = request.params["token"] || request.get_header("HTTP_X_API_TOKEN")
        @identifiers["ip"] = request.ip
        @identifiers["remote_addr"] = request.get_header("HTTP_REMOTE_ADDR")
        @identifiers["env"] = socket.env

        if @identifiers["current_user"]
          transmit(type: "welcome")
        else
          close
        end
      end

      def handle_close
        raise request.params["raise"] if request.params["raise"]

        TestFactory.log_event(
          "disconnect",
          name: @identifiers["current_user"],
          path: request.path,
          subscriptions: subscriptions
        )
      end

      def handle_channel_command(identifier, command, data)
        channel = channel_for(identifier)
        case command
        when "subscribe"
          channel.handle_subscribe
          !channel.subscription_rejected?
        when "unsubscribe"
          channel.handle_unsubscribe
          true
        when "message"
          channel.handle_action(data)
          true
        else
          raise "Unknown command"
        end
      end

      def transmit(data)
        socket.transmit data.to_json
      end

      def channel_for(identifier)
        channel_class = TestFactory.channel_for(identifier)
        channel_class&.new(self, identifier)
      end

      def identifiers_json
        @identifiers.to_json
      end

      def close
        socket.close
      end
    end

    class Channel
      attr_reader :connection, :identifier

      def initialize(connection, identifier)
        @connection = connection
        @identifier = identifier
      end

      def handle_subscribe; end

      def handle_unsubscribe; end

      def handle_action(data)
        decoded = JSON.parse(data)
        action = decoded.delete("action")
        public_send(action, decoded)
      end

      def subscription_rejected?
        @rejected == true
      end

      def stream_from(broadcasting)
        connection.socket.subscribe identifier, broadcasting
      end

      def stop_all_streams
        connection.socket.unsubscribe_from_all(identifier)
      end

      def transmit(msg)
        connection.transmit(identifier: identifier, data: msg)
      end
    end

    class << self
      def call(socket, **options)
        Connection.new(socket, options)
      end

      def register_channel(identifier, channel)
        channels[identifier] = channel
      end

      def channel_for(identifier)
        channels[identifier]
      end

      def events_log
        @events_log ||= []
      end

      def log_event(source, data)
        events_log << { source: source, data: data }
      end

      private

      def channels
        @channels ||= {}
      end
    end
  end
end
