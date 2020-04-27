# frozen_string_literal: true

module AnyCable
  module Middlewares
    # Checks that RPC client version is compatible with
    # the current RPC proto version
    class CheckVersion < Middleware
      attr_reader :version

      def initialize(version)
        @version = version
      end

      def call(_request, call, _method)
        supported_versions = call.metadata["protov"]&.split(",")
        return yield if supported_versions&.include?(version)

        raise GRPC::Internal,
          "Incompatible AnyCable RPC client.\nCurrent server version: #{version}.\n" \
          "Client supported versions: #{call.metadata["protov"] || "unknown"}."
      end
    end
  end
end
