# frozen_string_literal: true

module AnyCable
  module Middlewares
    # Checks that RPC client version is compatible with
    # the current RPC proto version
    class CheckVersion < AnyCable::Middleware
      attr_reader :version

      def initialize(version)
        @version = version
      end

      def call(_method, _request, meta)
        check_version(meta) do
          yield
        end
      end

      private

      def check_version(metadata)
        supported_versions = metadata["protov"]&.split(",")
        return yield if supported_versions&.include?(version)

        raise "Incompatible AnyCable RPC client.\nCurrent server version: #{version}.\n" \
              "Client supported versions: #{metadata["protov"] || "unknown"}."
      end
    end
  end
end
