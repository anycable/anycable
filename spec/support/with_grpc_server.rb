# frozen_string_literal: true

shared_context "with gRPC server", :with_grpc_server do
  before(:all) do
    @server = AnyCable::Server.new(
      host: AnyCable.config.rpc_host,
      **AnyCable.config.to_grpc_params
    )

    @server.start
  end

  after(:all) { @server.stop }
end
