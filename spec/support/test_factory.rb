module Anycable
  module TestFactory
    class Connection
      attr_reader :transmissions, :request, :identifiers, :subscriptions

      def initialize(env: nil, identifiers: nil, subscriptions: nil)
        @transmissions = []
        @identifiers = identifiers ? JSON.parse(identifiers) : {}
        @request = Rack::Request.new(env)
        @subscriptions = subscriptions
      end

      def handle_open
        @identifiers['current_user'] = request.cookies["username"]
        @identifiers['path'] = request.path
        @identifiers['token'] = request.params['token']

        if @identifiers['current_user']
          transmit(type: 'welcome')
        else
          @closed = true
        end
      end

      def handle_close
        TestFactory.log_event(
          'disconnect',
          name: @identifiers['current_user'],
          path: request.path,
          subscriptions: subscriptions
        )
      end

      def transmit(data)
        @transmissions << data.to_json
      end

      def channel_for(identifier)
        channel_class = TestFactory.channel_for(identifier)
        channel_class&.new(self, identifier)
      end

      def identifiers_json
        @identifiers.to_json
      end

      def closed?
        @closed == true
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
        action = decoded.delete('action')
        public_send(action, decoded)
      end

      def subscription_rejected?
        @rejected == true
      end

      def stream_from(broadcasting)
        streams << broadcasting
      end

      def stop_all_streams
        @stop_streams = true
      end

      def streams
        @streams ||= []
      end

      def stop_streams?
        @stop_streams == true
      end

      def transmit(msg)
        connection.transmit(identifier: identifier, data: msg)
      end
    end

    class << self
      def create(**options)
        Connection.new(options)
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
