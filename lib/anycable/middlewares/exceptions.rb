# frozen_string_literal: true

module AnyCable
  module Middlewares
    class Exceptions < AnyCable::Middleware
      def call(method_name, request)
        yield
      rescue => exp
        notify_exception(exp, method_name, request)

        response_class(method_name).new(
          status: AnyCable::Status::ERROR,
          error_msg: exp.message
        )
      end

      private

      def notify_exception(exp, method_name, message)
        AnyCable::ExceptionsHandling.notify(exp, method_name.to_s, message.to_h)
      end

      def response_class(method_name)
        case method_name
        when :connect
          AnyCable::ConnectionResponse
        when :disconnect
          AnyCable::DisconnectResponse
        else
          AnyCable::CommandResponse
        end
      end
    end
  end
end
