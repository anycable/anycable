# frozen_string_literal: true

require "json"
require "uri"
require "net/http"

module AnyCable
  module BroadcastAdapters
    # HTTP adapter for broadcasting.
    #
    # Example:
    #
    #   AnyCable.broadcast_adapter = :http
    #
    # It uses configuration from global AnyCable config
    # by default.
    #
    # You can override these params:
    #
    #   AnyCable.broadcast_adapter = :http, url: "http://ws.example.com/_any_cable_"
    class Http < Base
      # Taken from: https://github.com/influxdata/influxdb-ruby/blob/886058079c66d4fd019ad74ca11342fddb0b753d/lib/influxdb/errors.rb#L18
      RECOVERABLE_EXCEPTIONS = [
        Errno::ECONNABORTED,
        Errno::ECONNREFUSED,
        Errno::ECONNRESET,
        Errno::EHOSTUNREACH,
        Errno::EINVAL,
        Errno::ENETUNREACH,
        Net::HTTPBadResponse,
        Net::HTTPHeaderSyntaxError,
        Net::ProtocolError,
        SocketError,
        (OpenSSL::SSL::SSLError if defined?(OpenSSL))
      ].compact.freeze

      OPEN_TIMEOUT = 5
      READ_TIMEOUT = 10

      MAX_ATTEMPTS = 3
      DELAY = 2

      attr_reader :url, :headers, :authorization

      def initialize(url: AnyCable.config.http_broadcast_url, secret: AnyCable.config.broadcast_key)
        @url = url
        @headers = {}
        @authorization = nil

        if !secret
          secret = AnyCable.config.broadcast_key!
          @authorization = "with authorization key inferred from the application secret" if secret
        end

        if secret
          headers["Authorization"] = "Bearer #{secret}"
          @authorization ||= "with authorization"
        end

        @uri = URI.parse(url)
        @queue = Queue.new
      end

      def raw_broadcast(payload)
        ensure_thread_is_alive
        AnyCable.logger.info "Enqueue: #{payload}"
        queue << payload
      end

      # Wait for background thread to process all the messages
      # and stop it
      def shutdown
        queue << :stop
        thread.join if thread&.alive?
      rescue Exception => e # rubocop:disable Lint/RescueException
        logger.error "Broadcasting thread exited with exception: #{e.message}"
      end

      def announce!
        logger.info "Broadcasting HTTP url: #{url} (#{authorization || "no authorization"})"
      end

      private

      attr_reader :uri, :queue, :thread

      def ensure_thread_is_alive
        return if thread&.alive?

        @thread = Thread.new do
          loop do
            msg = queue.pop
            # @type break: nil
            break if msg == :stop

            handle_response perform_request(msg)
          end
        end
      end

      def perform_request(payload)
        build_http do |http|
          req = Net::HTTP::Post.new(url, {"Content-Type" => "application/json"}.merge(headers))
          req.body = payload
          AnyCable.logger.info "Perform: #{payload}"
          http.request(req)
        end
      end

      def handle_response(response)
        return unless response
        return if Net::HTTPCreated === response

        logger.error "Broadcast request responded with unexpected status: #{response.code}"
      end

      def build_http
        retry_count = 0

        begin
          http = Net::HTTP.new(uri.host, uri.port)
          http.open_timeout = OPEN_TIMEOUT
          http.read_timeout = READ_TIMEOUT
          http.use_ssl = url.match?(/^https/)
          http.verify_mode = OpenSSL::SSL::VERIFY_NONE
          yield http
        rescue Timeout::Error, *RECOVERABLE_EXCEPTIONS => e
          retry_count += 1
          if retry_count > MAX_ATTEMPTS
            logger.error("Broadcast request failed: #{e.message}")
            return
          end

          sleep((DELAY**retry_count).to_int * retry_count)
          retry
        ensure
          http.finish if http.started?
        end
      end
    end
  end
end
