# frozen_string_literal: true

module AnyCable
  module ExceptionsHandling # :nodoc:
    class << self
      def add_handler(block)
        handlers << procify(block)
      end

      alias << add_handler

      def notify(exp, method_name, message)
        handlers.each do |handler|
          begin
            handler.call(exp, method_name, message)
          rescue StandardError => exp
            AnyCable.logger.error "!!! EXCEPTION HANDLER THREW AN ERROR !!!"
            AnyCable.logger.error exp
            AnyCable.logger.error exp.backtrace.join("\n") unless exp.backtrace.nil?
          end
        end
      end

      private

      def procify(block)
        return block unless block.lambda?

        proc { |*args| block.call(*args.take(block.arity)) }
      end

      def handlers
        @handlers ||= []
      end
    end
  end
end
