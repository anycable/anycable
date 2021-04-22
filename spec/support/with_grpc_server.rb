# frozen_string_literal: true

RSpec.shared_context "anycable:grpc:server" do
  before(:all) do
    @server = AnyCable::GRPC::Server.new(
      host: AnyCable.config.rpc_host,
      **AnyCable.config.to_grpc_params
    )

    @server.start
    sleep 0.1
  end

  after(:all) { @server.stop }
end
