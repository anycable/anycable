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

      alias [] read

      def write(key, val)
        return if source&.[](key) == val

        @source ||= {}
        @dirty_keys ||= []
        dirty_keys << key
        source[key] = val
      end

      alias []= write

      def changed_fields
        return unless source && dirty_keys
        source.slice(*dirty_keys)
      end
    end

    attr_reader :transmissions, :env, :cstate, :istate

    def initialize(env: nil)
      @transmissions = []
      @env = env
      @cstate = env["anycable.cstate"] = State.new(env["anycable.raw_cstate"])
      @istate = env["anycable.istate"] = State.new(env["anycable.raw_istate"])
    end

    def transmit(websocket_message)
      transmissions << websocket_message
    end

    def subscribe(_channel, broadcasting)
      streams[:start] << broadcasting
    end

    def unsubscribe(_channel, broadcasting)
      streams[:stop] << broadcasting
    end

    def unsubscribe_from_all(_channel)
      @stop_all_streams = true
    end

    def streams
      @streams ||= {start: [], stop: []}
    end

    def close
      @closed = true
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
