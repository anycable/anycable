# frozen_string_literal: true

module AnyCable
  # Socket mock to be used with application connection
  class Socket
    # Represents the per-connection store
    # (for example, used to keep session beetween RPC calls)
    class State
      attr_reader :dirty_keys, :source

      def initialize(from)
        @source = from
        @dirty_keys = nil
      end

      def read(key)
        source&.[](key)
      end

      def write(key, val)
        return if source&.[](key) == val

        @source ||= {}
        @dirty_keys ||= []
        dirty_keys << key
        source[key] = val
      end

      def changed_fields
        return unless source && dirty_keys
        source.slice(*dirty_keys)
      end
    end

    attr_reader :transmissions, :env, :cstate

    def initialize(env: nil)
      @transmissions = []
      @env = env
      @cstate = env["anycable.cstate"] = State.new(env["anycable.raw_cstate"])
    end

    def transmit(websocket_message)
      transmissions << websocket_message
    end

    def subscribe(_channel, broadcasting)
      streams << broadcasting
    end

    def unsubscribe(_channel, _broadcasting)
      raise NotImplementedError
    end

    def unsubscribe_from_all(_channel)
      @stop_all_streams = true
    end

    def streams
      @streams ||= []
    end

    def close
      @closed = true
      @transmissions.clear
      @streams&.clear
      @stop_all_streams = true
    end

    def closed?
      @closed == true
    end

    def stop_streams?
      @stop_all_streams == true
    end

    def session
      cstate.read(SESSION_KEY)
    end

    def session=(val)
      cstate.write(SESSION_KEY, val)
    end
  end
end
