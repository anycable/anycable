# frozen_string_literal: true

module AnyCable
  module ExceptionsHandling # :nodoc:
    class << self
      def add_handler(block)
        handlers << block
      end

      alias << add_handler

      def notify(exp)
        handlers.each do |handler|
          begin
            handler.call(exp)
          rescue StandardError => exp
            AnyCable.logger.error "!!! EXCEPTION HANDLER THREW AN ERROR !!!"
            AnyCable.logger.error exp
            AnyCable.logger.error exp.backtrace.join("\n") unless exp.backtrace.nil?
          end
        end
      end

      private

      def handlers
        @handlers ||= []
      end
    end
  end
end
