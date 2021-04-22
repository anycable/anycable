# frozen_string_literal: true

module AnyCable
  module GRPC
    # Checks that gRPC client version is compatible with
    # the current RPC proto version
    class CheckVersion < ::GRPC::ServerInterceptor
      attr_reader :version

      def initialize(version)
        @version = version
      end

      def request_response(request: nil, call: nil, method: nil)
        # Call only for AnyCable service
        return yield unless method.receiver.is_a?(AnyCable::GRPC::Handler)

        check_version(call) do
          yield
        end
      end

      def check_version(call)
        supported_versions = call.metadata["protov"]&.split(",")
        return yield if supported_versions&.include?(version)

        raise ::GRPC::Internal,
          "Incompatible AnyCable RPC client.\nCurrent server version: #{version}.\n" \
          "Client supported versions: #{call.metadata["protov"] || "unknown"}."
      end
    end
  end
end
