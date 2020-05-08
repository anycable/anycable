# frozen_string_literal: true

module AnyCable
  module BroadcastAdapters
    class Base
      def broadcast(_stream, _payload)
        raise NotImplementedError
      end

      def announce!
        logger.info "Broadcasting via #{self.class.name}"
      end

      private

      def logger
        AnyCable.logger
      end
    end
  end
end
