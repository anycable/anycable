# frozen_string_literal: true

shared_context "with gRPC server", :with_grpc_server do
  before(:all) do
    @server = Anycable::Server.new(
      host: Anycable.config.rpc_host,
      **Anycable.config.to_grpc_params
    )

    @server.start
  end

  after(:all) { @server.stop }
end
