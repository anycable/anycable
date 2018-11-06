# frozen_string_literal: true

module AnyCable
  # Socket mock to be used with application connection
  class Socket
    attr_reader :transmissions, :env

    def initialize(env: nil)
      @transmissions = []
      @env = env
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
  end
end
