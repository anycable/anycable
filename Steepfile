# frozen_string_literal: true

target :lib do
  signature "sig"
  check "lib"

  configure_code_diagnostics do |hash|
    hash[Steep::Diagnostic::Ruby::UnknownInstanceVariable] = :information
  end

  ignore "lib/anycable/rspec/*.rb"
  ignore "lib/anycable/grpc/rpc_services_pb.rb"
  ignore "lib/anycable/protos/*.rb"
  # TODO: unignore
  ignore "lib/anycable/grpc_kit/*.rb"

  ignore "lib/anycable/cli.rb"

  # Splat args not supported
  ignore "lib/anycable/exceptions_handling.rb"
  ignore "lib/anycable/broadcast_adapters/http.rb"
end
