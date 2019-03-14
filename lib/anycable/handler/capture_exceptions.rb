# frozen_string_literal: true

require "anycable/rpc/rpc_pb"

module AnyCable
  module Handler # :nodoc:
    # Handle app-level errors.
    #
    # NOTE: this functionality couldn't be implemeted
    # as middleware, 'cause interceptors do not support
    # aborting the call and returning a data
    module CaptureExceptions
      RESPONSE_CLASS = {
        command: AnyCable::CommandResponse,
        connect: AnyCable::ConnectionResponse,
        disconnect: AnyCable::DisconnectResponse
      }.freeze

      RESPONSE_CLASS.keys.each do |mid|
        module_eval <<~CODE, __FILE__, __LINE__ + 1
          def #{mid}(message, *)
            capture_exceptions(:#{mid}, message) { super }
          end
        CODE
      end

      def capture_exceptions(method_name, message)
        yield
      rescue StandardError => exp
        AnyCable::ExceptionsHandling.notify(exp, method_name.to_s, message.to_h)

        RESPONSE_CLASS.fetch(method_name).new(
          status: AnyCable::Status::ERROR,
          error_msg: exp.message
        )
      end
    end
  end
end
