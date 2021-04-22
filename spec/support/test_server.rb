# frozen_string_literal: true

# Dummy server implementation for CLI testing
module AnyCable
  class TestServer
    class << self
      def call(_)
        AnyCable.logger.info "Dummy RPC is running!"
        TestServer.new
      end
    end

    def start
    end

    def stop
    end

    def running?
      true
    end
  end
end
