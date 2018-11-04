# frozen_string_literal: true

module Anycable
  module ExceptionsHandling # :nodoc:
    class << self
      def add_handler(block)
        @handlers ||= []
        @handlers << block
      end

      alias << add_handler

      def notify(exp)
        @handlers.each do |handler|
          begin
            handler.call(exp)
          rescue StandardError => exp
            Anycable.logger.error "!!! EXCEPTION HANDLER THREW AN ERROR !!!"
            Anycable.logger.error exp
            Anycable.logger.error exp.backtrace.join("\n") unless exp.backtrace.nil?
          end
        end
      end
    end
  end
end
