# frozen_string_literal: true

RSpec.shared_context "anycable:grpc:server" do
  before(:all) do
    @server = AnyCable.server_builder.call(AnyCable.config)

    @server.start
    sleep 0.1
  end

  after(:all) { @server.stop }
end
