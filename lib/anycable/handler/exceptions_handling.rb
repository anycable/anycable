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
        Anycable.error_handlers.each do |handler|
          begin
            handler.call(exp)
          rescue StandardError => exp
            Anycable.logger.error "!!! ERROR HANDLER THREW AN ERROR !!!"
            Anycable.logger.error exp
            Anycable.logger.error exp.backtrace.join("\n") unless exp.backtrace.nil?
          end
        end
      end
    end

    Anycable.error_handlers << proc { |e| Anycable.logger.error(e.message) }
  end
end
