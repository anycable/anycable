# frozen_string_literal: true

module Anycable
  module Handler
    # Handle app-level errors
    module ExceptionsHandling
      def connect(*)
        super
      rescue StandardError => e
        logger.error(e.message)
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR)
      end

      def disconnect(*)
        super
      rescue StandardError => e
        logger.error(e.message)
        Anycable::DisconnectResponse.new(status: Anycable::Status::ERROR)
      end

      def command(*)
        super
      rescue StandardError => e
        logger.error(e.message)
        Anycable::CommandResponse.new(status: Anycable::Status::ERROR)
      end
    end
  end
end
