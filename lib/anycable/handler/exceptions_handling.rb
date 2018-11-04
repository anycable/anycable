# frozen_string_literal: true

module Anycable
  module Handler # :nodoc:
    # Handle app-level errors
    module ExceptionsHandling
      def connect(*)
        super
      rescue StandardError => exp
        handle_exception(exp)
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR, error_msg: exp.message)
      end

      def disconnect(*)
        super
      rescue StandardError => exp
        handle_exception(exp)
        Anycable::DisconnectResponse.new(status: Anycable::Status::ERROR, error_msg: exp.message)
      end

      def command(*)
        super
      rescue StandardError => exp
        handle_exception(exp)
        Anycable::CommandResponse.new(status: Anycable::Status::ERROR, error_msg: exp.message)
      end

      def handle_exception(exp)
        Anycable::ExceptionsHandling.notify(exp)
      end
    end
  end
end
