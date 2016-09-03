# frozen_string_literal: true
require "action_cable"

module ActionCable
  module Connection
    class Base # :nodoc:
      attr_reader :transmissions

      def initialize(env: {}, identifiers_json: '{}')
        @ids = ActiveSupport::JSON.decode(identifiers_json)
        @cached_ids = {}
        @env = env
        @coder = ActiveSupport::JSON
        @closed = false
        @transmissions = []
        @subscriptions = ActionCable::Connection::Subscriptions.new(self)
      end

      def handle_open
        connect if respond_to?(:connect)
        send_welcome_message
      rescue ActionCable::Connection::Authorization::UnauthorizedError
        close
      end

      def handle_close
        # subscriptions.unsubscribe_from_all
        disconnect if respond_to?(:disconnect)
      end

      def close
        @closed = true
      end

      def closed?
        @closed
      end

      def transmit(cable_message)
        transmissions << encode(cable_message)
      end

      def dispose
        @closed = false
        transmissions.clear
      end

      # Generate identifiers info.
      # Converts GlobalID compatible vars to corresponding global IDs params.
      def identifiers_hash
        identifiers.each_with_object({}) do |id, acc|
          obj = instance_variable_get("@#{id}")
          next unless obj
          acc[id] = obj.try(:to_gid_param) || obj
        end
      end

      # Fetch identifier and deserialize if neccessary
      def fetch_identifier(name)
        @cached_ids[name] ||= @cached_ids.fetch(name) do
          val = @ids[name.to_s]
          next val unless val.is_a?(String)
          GlobalID::Locator.locate(val) || val
        end
      end

      def logger
        ::Rails.logger
      end
    end
  end
end
