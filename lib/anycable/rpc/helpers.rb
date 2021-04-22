# frozen_string_literal: true

# Common helper methods for RPC handlers (extracted from the old rpc_helper.rb)
module AnyCable
  module RPC
    module Helpers
      # Build Rack env from request
      def rack_env(request_env)
        uri = URI.parse(request_env.url)

        env = base_rack_env
        env.merge!({
          "PATH_INFO" => uri.path,
          "QUERY_STRING" => uri.query,
          "SERVER_NAME" => uri.host,
          "SERVER_PORT" => uri.port,
          "HTTP_HOST" => uri.host,
          "REMOTE_ADDR" => request_env.headers.delete("REMOTE_ADDR"),
          "rack.url_scheme" => uri.scheme&.sub(/^ws/, "http"),
          # AnyCable specific fields
          "anycable.raw_cstate" => request_env.cstate&.to_h,
          "anycable.raw_istate" => request_env.istate&.to_h
        }.delete_if { |_k, v| v.nil? })

        env.merge!(build_headers(request_env.headers))
      end

      def base_rack_env
        # Minimum required variables according to Rack Spec
        # (not all of them though, just those enough for Action Cable to work)
        # See https://rubydoc.info/github/rack/rack/master/file/SPEC
        # and https://github.com/rack/rack/blob/master/lib/rack/lint.rb
        {
          "REQUEST_METHOD" => "GET",
          "SCRIPT_NAME" => "",
          "PATH_INFO" => "/",
          "QUERY_STRING" => "",
          "SERVER_NAME" => "",
          "SERVER_PORT" => "80",
          "rack.url_scheme" => "http",
          "rack.input" => StringIO.new("", "r").tap { |io| io.set_encoding(Encoding::ASCII_8BIT) },
          "rack.version" => ::Rack::VERSION,
          "rack.errors" => StringIO.new("").tap { |io| io.set_encoding(Encoding::ASCII_8BIT) },
          "rack.multithread" => true,
          "rack.multiprocess" => false,
          "rack.run_once" => false,
          "rack.hijack?" => false
        }
      end

      def build_socket(**options)
        AnyCable::Socket.new(**options)
      end

      def build_headers(headers)
        headers.each_with_object({}) do |(k, v), obj|
          k = k.upcase
          k.tr!("-", "_")
          obj["HTTP_#{k}"] = v
        end
      end

      def build_env_response(socket)
        AnyCable::EnvResponse.new(
          cstate: socket.cstate.changed_fields,
          istate: socket.istate.changed_fields
        )
      end

      def logger
        AnyCable.logger
      end

      def factory
        AnyCable.connection_factory
      end
    end
  end
end
