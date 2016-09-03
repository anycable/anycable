# frozen_string_literal: true
require "action_cable"

module ActionCable
  module Channel
    class Base # :nodoc:
      alias do_subscribe subscribe_to_channel

      public :do_subscribe, :subscription_rejected?

      def subscribe_to_channel
        # noop
      end

      attr_reader :stop_streams

      def stream_from(broadcasting, callback = nil, coder: nil)
        raise ArgumentError('Unsupported') if callback.present? || coder.present? || block_given?
        streams << broadcasting
      end

      def stop_all_streams
        @stop_streams = true
      end

      def streams
        @streams ||= []
      end

      def stop_streams?
        stop_streams == true
      end

      def delegate_connection_identifiers
        connection.identifiers.each do |identifier|
          define_singleton_method(identifier) do
            connection.fetch_identifier(identifier)
          end
        end
      end
    end
  end
end
