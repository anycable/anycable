# frozen_string_literal: true

require "async"
require "async/http/endpoint"
require "async/http/server"
require "async/websocket/client"
require "async/websocket/adapters/http"
require "open-uri"

class MockWebSocketServer
  attr_reader :port, :socket, :endpoint, :token

  def initialize(port: nil, token: nil)
    @port = port || 8485
    @token = token
    @messages = []
    @has_connection = Queue.new
    @endpoint = Async::HTTP::Endpoint.parse("http://127.0.0.1:#{@port}", alpn_protocols: Async::HTTP::Protocol::HTTP11.names)
  end

  def ws_endpoint
    Async::HTTP::Endpoint.parse("ws://#{endpoint.hostname}:#{endpoint.port}")
  end

  def read_message(timeout: 1.0)
    loop do
      return @messages.shift unless @messages.empty?

      timeout -= 0.1
      sleep 0.1
      raise "No message received" if timeout <= 0
    end
  end

  def send_message(msg)
    @connection&.write(msg)
    @connection&.flush
  end

  def start
    app = proc do |request|
      if request.path == "/health"
        next Protocol::HTTP::Response[200, {}, ["ok"]]
      end

      Async::WebSocket::Adapters::HTTP.open(request) do |connection|
        @connection = connection
        @has_connection << connection

        if token && !request.path.include?(%(token=#{token}))
          send_message({type: "disconnect", reason: "unauthorized", reconnect: false}.to_json)
          connection.close
          next
        end

        send_message({type: "connect"}.to_json)

        while message = @connection.read # rubocop:disable Lint/AssignmentInCondition
          @messages << message
        end
      ensure
        @connection = nil
      end
    end

    @server = Async::HTTP::Server.new(app, endpoint)

    @server_task = Thread.new do
      Async do
        @server.run
      end
    end

    wait_server_ready
  end

  def stop
    send_message({type: "disconnect", reason: "server_restart", reconnect: true}.to_json)
    @server_task&.terminate

    @server_task = nil
    @connection = nil
    @server = nil
  end

  def ensure_has_connection
    @has_connection.pop(timeout: 2.0).tap do |val|
      raise "No connection established" unless val
    end
  end

  def wait_server_ready(timeout: 2.0)
    loop do
      URI.open("http://#{endpoint.hostname}:#{endpoint.port}/health", &:read)
      break
    rescue => e
      timeout -= 0.1
      sleep 0.1
      raise "Server not ready: #{e.class} — #{e.message}" if timeout <= 0
    end
  end
end
