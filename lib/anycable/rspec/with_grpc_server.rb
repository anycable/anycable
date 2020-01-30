# frozen_string_literal: true

RSpec.shared_context "anycable:rpc:server" do
  before(:all) do
    @server = AnyCable::Server.new(
      host: AnyCable.config.rpc_host,
      **AnyCable.config.to_grpc_params,
      interceptors: AnyCable.middleware.to_a
    )

    @server.start
  end

  after(:all) { @server.stop }
end
