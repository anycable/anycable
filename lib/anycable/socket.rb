# frozen_string_literal: true

require "stringio"

module AnyCable
  WHISPER_KEY = "$w"

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

      alias_method :[], :read

      def write(key, val)
        return if source&.[](key) == val

        @source ||= {}

        keys = (@dirty_keys ||= [])
        keys << key

        source[key] = val
      end

      alias_method :[]=, :write

      def changed_fields
        return unless source

        keys = dirty_keys
        return if keys.nil?

        source.slice(*keys)
      end
    end

    attr_reader :transmissions

    def initialize(env:)
      @transmissions = []
      @request_env = env
    end

    def transmit(websocket_message)
      transmissions << websocket_message
    end

    def subscribe(_channel, broadcasting)
      streams[:start] << broadcasting
    end

    def unsubscribe(_channel, broadcasting)
      streams[:stop] << broadcasting

      if istate.read(WHISPER_KEY) == broadcasting
        istate.write(WHISPER_KEY, "")
      end
    end

    def whisper(_channel, broadcasting)
      istate.write(WHISPER_KEY, broadcasting)
    end

    def unsubscribe_from_all(_channel)
      @stop_all_streams = true
      if istate.read(WHISPER_KEY)
        istate.write(WHISPER_KEY, "")
      end
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

    def env
      return @env if defined?(@env)

      @env = build_rack_env
    end

    def istate
      return @istate if defined?(@istate)

      @istate = env["anycable.istate"] = State.new(env["anycable.raw_istate"])
    end

    def cstate
      return @cstate if defined?(@cstate)

      @cstate = env["anycable.cstate"] = State.new(env["anycable.raw_cstate"])
    end

    private

    attr_reader :request_env

    # Build Rack env from request
    def build_rack_env
      uri = URI.parse(request_env.url)
      headers = request_env.headers.to_h

      env = base_rack_env
      env.merge!({
        "PATH_INFO" => uri.path,
        "QUERY_STRING" => uri.query,
        "SERVER_NAME" => uri.host,
        "SERVER_PORT" => uri.port&.to_s,
        "HTTP_HOST" => uri.host,
        "REMOTE_ADDR" => headers.delete("REMOTE_ADDR"),
        "rack.url_scheme" => uri.scheme&.sub(/^ws/, "http"),
        # AnyCable specific fields
        "anycable.raw_cstate" => request_env.cstate&.to_h,
        "anycable.raw_istate" => request_env.istate&.to_h,
        "anycable.sid" => request_env.sid
      }.delete_if { |_k, v| v.nil? })

      env.merge!(build_headers(headers))
    end

    def base_rack_env
      # Minimum required variables according to Rack Spec
      # (not all of them though, just those enough for Action Cable to work)
      # See https://rubydoc.info/github/rack/rack/master/file/SPEC
      # and https://github.com/rack/rack/blob/master/lib/rack/lint.rb
      env = {
        "REQUEST_METHOD" => "GET",
        "SCRIPT_NAME" => "",
        "PATH_INFO" => "/",
        "QUERY_STRING" => "",
        "SERVER_NAME" => "",
        "SERVER_PORT" => "80",
        "SERVER_PROTOCOL" => "HTTP/1.1",
        "rack.url_scheme" => "http",
        "rack.input" => StringIO.new("", "r").tap { |io| io.set_encoding(Encoding::ASCII_8BIT) },
        "rack.errors" => StringIO.new("").tap { |io| io.set_encoding(Encoding::ASCII_8BIT) },
        "rack.multithread" => true,
        "rack.multiprocess" => false,
        "rack.run_once" => false,
        "rack.hijack?" => false
      }

      # Rack 3.1 removes `Rack::VERSION`. rack.version is optional (deprecated) since Rack 3.0
      if ::Rack::RELEASE < "3.0"
        rversion = ::Rack::VERSION
        # @type var rversion : String
        env["rack.version"] = rversion
      end
      env
    end

    def build_headers(headers)
      headers.each_with_object({}) do |header, obj|
        k, v = *header
        k = k.upcase
        k.tr!("-", "_")
        obj["HTTP_#{k}"] = v
      end
    end
  end
end
