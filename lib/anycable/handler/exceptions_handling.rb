# frozen_string_literal: true

module Anycable
  module Handler # :nodoc:
    # Handle app-level errors
    module ExceptionsHandling
      def connect(*)
        super
      rescue StandardError => ex
        handle_exception(ex)
        Anycable::ConnectionResponse.new(status: Anycable::Status::ERROR, error_msg: ex.message)
      end

      def disconnect(*)
        super
      rescue StandardError => ex
        handle_exception(ex)
        Anycable::DisconnectResponse.new(status: Anycable::Status::ERROR, error_msg: ex.message)
      end

      def command(*)
        super
      rescue StandardError => ex
        handle_exception(ex)
        Anycable::CommandResponse.new(status: Anycable::Status::ERROR, error_msg: ex.message)
      end

      def handle_exception(ex)
        Anycable.error_handlers.each do |handler|
          begin
            handler.call(ex)
          rescue StandardError => ex
            Anycable.logger.error "!!! ERROR HANDLER THREW AN ERROR !!!"
            Anycable.logger.error ex
            Anycable.logger.error ex.backtrace.join("\n") unless ex.backtrace.nil?
          end
        end
      end
    end

    Anycable.error_handlers << proc { |e| Anycable.logger.error(e.message) }
  end
end
