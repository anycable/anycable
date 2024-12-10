# frozen_string_literal: true

AnyCable::Config.attr_config(
  ### gRPC options
  rpc_host: "127.0.0.1:50051",
  rpc_tls_cert: nil,
  rpc_tls_key: nil,
  rpc_max_waiting_requests: 20,
  rpc_poll_period: 1,
  rpc_pool_keep_alive: 0.25,
  rpc_server_args: {},
  rpc_max_connection_age: 300,
  log_grpc: false
)

AnyCable::Config.ignore_options :rpc_server_args
AnyCable::Config.flag_options :log_grpc

module AnyCable
  module GRPC
    module Config
      def log_grpc
        debug? || super
      end

      # Add alias explicitly, 'cause previous alias refers to the original log_grpc method
      alias_method :log_grpc?, :log_grpc

      # Build gRPC server parameters
      def to_grpc_params
        {
          pool_size: rpc_pool_size,
          max_waiting_requests: rpc_max_waiting_requests,
          poll_period: rpc_poll_period,
          pool_keep_alive: rpc_pool_keep_alive,
          tls_credentials: tls_credentials,
          server_args: enhance_grpc_server_args(normalized_grpc_server_args).tap do |sargs|
            # Provide keepalive defaults unless explicitly set.
            # They must MATCH the corresponding Go client defaults:
            # https://github.com/anycable/anycable-go/blob/62e77e7f759aa9253c2bd23812dd59ec8471db86/rpc/rpc.go#L512-L515
            #
            # See also https://github.com/grpc/grpc/blob/master/doc/keepalive.md and https://grpc.github.io/grpc/core/group__grpc__arg__keys.html
            sargs["grpc.keepalive_permit_without_calls"] ||= 1
            sargs["grpc.http2.min_recv_ping_interval_without_data_ms"] ||= 10_000
          end
        }
      end

      def normalized_grpc_server_args
        val = rpc_server_args
        return {} unless val.is_a?(Hash)

        val.transform_keys do |key|
          skey = key.to_s
          skey.start_with?("grpc.") ? skey : "grpc.#{skey}"
        end
      end

      def enhance_grpc_server_args(opts)
        return opts if opts.key?("grpc.max_connection_age_ms")
        return opts unless rpc_max_connection_age.to_i > 0

        opts["grpc.max_connection_age_ms"] = rpc_max_connection_age.to_i * 1000
        opts
      end

      def tls_credentials
        cert_path_or_content = rpc_tls_cert # Assign to local variable to make steep happy
        key_path_or_content = rpc_tls_key # Assign to local variable to make steep happy
        return {} if cert_path_or_content.nil? || key_path_or_content.nil?

        cert = File.exist?(cert_path_or_content) ? File.read(cert_path_or_content) : cert_path_or_content
        pkey = File.exist?(key_path_or_content) ? File.read(key_path_or_content) : key_path_or_content

        {cert: cert, pkey: pkey}
      end
    end
  end
end

AnyCable::Config.prepend AnyCable::GRPC::Config

AnyCable::Config.usage <<~TXT
  GRPC OPTIONS
      --rpc-host=host                   Local address to run gRPC server on, default: "127.0.0.1:50051"
      --rpc-tls-cert=path               TLS certificate file path or contents in PEM format, default: <none> (TLS disabled)
      --rpc-tls-key=path                TLS private key file path or contents in PEM format, default: <none> (TLS disabled)
      --rpc-pool-size=size              gRPC workers pool size, default: 30
      --rpc-max-waiting-requests=num    Max waiting requests queue size, default: 20
      --rpc-poll-period=seconds         Poll period (sec), default: 1
      --rpc-pool-keep-alive=seconds     Keep-alive polling interval (sec), default: 1
      --log-grpc                        Enable gRPC logging (disabled by default)
TXT
