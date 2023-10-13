# frozen_string_literal: true

module AnyCable
  module BroadcastAdapters
    class Base
      def raw_broadcast(_data)
        raise NotImplementedError
      end

      def batching(enabled = true)
        self.batching_enabled = enabled
        yield
      ensure
        maybe_flush_batch
      end

      def start_batching
        self.batching_enabled = true
      end

      def finish_batching
        maybe_flush_batch
      end

      def batching?
        Thread.current[:anycable_batching]&.last
      end

      def broadcast(stream, payload, options = nil)
        if batching?
          current_batch << {stream: stream, data: payload, meta: options}.compact
        else
          raw_broadcast({stream: stream, data: payload, meta: options}.compact.to_json)
        end
      end

      def broadcast_command(command, **payload)
        raw_broadcast({command: command, payload: payload}.to_json)
      end

      def announce!
        logger.info "Broadcasting via #{self.class.name}"
      end

      private

      def batching_enabled=(val)
        # The stack must start with the true value,
        # so we can check for emptiness to decide whether to flush
        stack = batching_enabled_stack
        stack << val if val || !stack.empty?
      end

      def batching_enabled_stack
        Thread.current[:anycable_batching] ||= []
      end

      def current_batch
        Thread.current[:anycable_batch] ||= []
      end

      def maybe_flush_batch
        batching_enabled_stack.pop
        return unless batching_enabled_stack.empty?

        batch = current_batch
        unless batch.empty?
          raw_broadcast(batch.to_json)
        end
        current_batch.clear
      end

      def logger
        AnyCable.logger
      end
    end
  end
end
