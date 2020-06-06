# frozen_string_literal: true

module AnyCable
  module BroadcastAdapters
    class Base
      def raw_broadcast(_data)
        raise NotImplementedError
      end

      def broadcast(stream, payload)
        raw_broadcast({stream: stream, data: payload}.to_json)
      end

      def broadcast_command(command, **payload)
        raw_broadcast({command: command, payload: payload}.to_json)
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
