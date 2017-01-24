# frozen_string_literal: true
module Anycable
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

    def stream(broadcasting)
      streams << broadcasting
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

    def stop_all_streams
      @stop_all_streams = true
    end

    def stop_streams?
      @stop_all_streams == true
    end
  end
end
