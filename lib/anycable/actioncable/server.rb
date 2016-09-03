# frozen_string_literal: true
require "action_cable"
require "anycable/pubsub"

module ActionCable
  module Server
    # Override pubsub for ActionCable
    class Base
      def pubsub
        @any_pubsub ||= Anycable::PubSub.new
      end
    end
  end
end
