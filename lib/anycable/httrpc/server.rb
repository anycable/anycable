# frozen_string_literal: true

module AnyCable
  module HTTRPC
    class Server
      def initialize(token: AnyCable.config.http_rpc_secret)
        if !token
          token = AnyCable.config.http_rpc_secret!
          AnyCable.logger.info("AnyCable HTTP RPC created with authorization key inferred from the application secret")
        end

        @token = token
      end

      def call(env)
        if env["REQUEST_METHOD"] != "POST"
          return [404, {}, ["Not found"]]
        end

        if token && env["HTTP_AUTHORIZATION"] != "Bearer #{token}"
          return [401, {}, ["Unauthorized"]]
        end

        rpc_command = env["PATH_INFO"].sub(%r{^/}, "").to_sym

        # check if command is supported and return 404 if not
        return [404, {}, ["Not found"]] unless AnyCable.rpc_handler.supported?(rpc_command)

        # read request body and it's empty, return 422
        request_body = env["rack.input"].read
        return [422, {}, ["Empty request body"]] if request_body.empty?

        payload =
          case rpc_command
          when :connect then AnyCable::ConnectionRequest.decode_json(request_body)
          when :disconnect then AnyCable::DisconnectRequest.decode_json(request_body)
          when :command then AnyCable::CommandMessage.decode_json(request_body)
          end

        return [422, {}, ["Invalid request body"]] if payload.nil?

        response = AnyCable.rpc_handler.handle(rpc_command, payload, build_meta(env))

        [200, {}, [response.to_json({format_enums_as_integers: true, preserve_proto_fieldnames: true})]]
      rescue JSON::ParserError, ArgumentError => e
        [422, {}, ["Invalid request body: #{e.message}"]]
      end

      private

      attr_reader :token

      def build_meta(env)
        # `x-anycable-meta-var` -> `meta["var"]`
        env.each_with_object({}) do |(k, v), meta|
          next unless k.start_with?("HTTP_X_ANYCABLE_META_")
          meta[k.sub(%r{^HTTP_X_ANYCABLE_META_}, "").downcase] = v
          meta
        end
      end
    end
  end
end
