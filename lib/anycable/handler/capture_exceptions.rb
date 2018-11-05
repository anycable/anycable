# frozen_string_literal: true

require "anycable/rpc/rpc_pb"

module Anycable
  module Handler # :nodoc:
    # Handle app-level errors.
    #
    # NOTE: this functionality couldn't be implemeted
    # as middleware, 'cause interceptors do not support
    # aborting the call and returning a data
    module CaptureExceptions
      RESPONSE_CLASS = {
        command: Anycable::CommandResponse,
        connect: Anycable::ConnectionResponse,
        disconnect: Anycable::DisconnectResponse
      }.freeze

      RESPONSE_CLASS.keys.each do |mid|
        module_eval <<~CODE, __FILE__, __LINE__ + 1
          def #{mid}(*)
            capture_exceptions(:#{mid}) { super }
          end
        CODE
      end

      def capture_exceptions(method_name)
        yield
      rescue StandardError => exp
        Anycable::ExceptionsHandling.notify(exp)

        RESPONSE_CLASS.fetch(method_name).new(
          status: Anycable::Status::ERROR,
          error_msg: exp.message
        )
      end
    end
  end
end
