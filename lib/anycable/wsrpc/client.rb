# frozen_string_literal: true

begin
  require "async"
  require "async/http/endpoint"
  require "async/websocket/client"
rescue LoadError => e
  warn "Make sure async libraries are installed to use WebSocket RPC: #{e.message}"
  raise
end

module AnyCable
  module WSRPC
    class Client
      def initialize(app, url, max_reconnects: AnyCable.config.ws_rpc_max_reconnects, logger: AnyCable.logger)
        @endpoint = Async::HTTP::Endpoint.parse(url)
        @logger = logger
        @app = app
        @main_thread = nil
        @queue = Async::Queue.new
        @open = false
        @closed = false
        @attempt = 0
        @max_reconnects = max_reconnects
      end

      def send_message(msg)
        @queue.push(msg)
      end

      alias_method :<<, :send_message

      def open
        @closed = false
        @main_thread = Thread.new do
          loop do
            @main_task = Async do |task|
              Async::WebSocket::Client.connect(@endpoint) do |connection|
                @open = true
                @attempt = 0

                @logger.info("WebSocket RPC client connected")

                input_task = task.async do
                  while msg = @queue.pop # rubocop:disable Lint/AssignmentInCondition
                    connection.write(msg)
                    connection.flush
                  end
                end

                while message = connection.read # rubocop:disable Lint/AssignmentInCondition
                  @app.call(self, message)
                end
              ensure
                @open = false
                input_task&.stop
              end
            end

            @main_task.wait
            break unless maybe_reconnect
          end
        end

        @main_thread.abort_on_exception = true
        @main_thread
      end

      def open?
        @open
      end

      def closed?
        !@open
      end

      def close
        # Indicate that we want to close the connection,
        # so no reconnect is required
        @closed = true

        @main_task&.terminate
        @main_thread&.terminate
      end

      private

      def maybe_reconnect
        return @logger.info("WebSocket RPC client disconnected") if @closed

        @logger.info("WebSocket RPC connection lost")

        @attempt += 1

        if @attempt > @max_reconnects
          raise "Max reconnects reached for WS RPC"
        end

        delay_base = [2**@attempt, 30].min
        # @type var delay_base: Integer
        delay_seconds = delay_base + rand

        @logger.info("WebSocket RPC reconnecting in #{delay_seconds} seconds...")

        # Sleep before attempting reconnect
        sleep(delay_seconds)

        true
      end
    end
  end
end
