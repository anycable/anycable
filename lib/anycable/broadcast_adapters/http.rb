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
    #   AnyCable.broadast_adapter = :http
    #
    # It uses configuration from global AnyCable config
    # by default.
    #
    # You can override these params:
    #
    #   AnyCable.broadcast_adapter = :http, url: "http://ws.example.com/_any_cable_"
    class Http
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

      attr_reader :url

      def initialize(url: AnyCable.config.http_broadcast_url)
        @url = url
        @uri = URI.parse(url)
      end

      def broadcast(stream, payload)
        payload = {stream: stream, data: payload}.to_json

        build_http do |http|
          req = Net::HTTP::Post.new(url, {"Content-Type" => "application/json"})
          req.body = payload
          http.request(req)
        end
      end

      private

      attr_reader :uri

      def build_http
        retry_count = 0

        begin
          http = Net::HTTP.new(uri.host, uri.port)
          http.open_timeout = OPEN_TIMEOUT
          http.read_timeout = READ_TIMEOUT
          yield http
        rescue Timeout::Error, *RECOVERABLE_EXCEPTIONS
          retry_count += 1
          raise if MAX_ATTEMPTS < retry_count

          sleep((DELAY**retry_count) * retry_count)
          retry
        ensure
          http.finish if http.started?
        end
      end
    end
  end
end
