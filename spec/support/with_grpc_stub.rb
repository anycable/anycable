# frozen_string_literal: true

RSpec.shared_context "anycable:grpc:stub" do
  include_context "rpc_command"

  before(:all) do
    @service =
      if GRPC_KIT
        sock = TCPSocket.new(*AnyCable.config.rpc_host.split(":"))
        AnyCable::GRPC::Stub.new(sock)
      else
        AnyCable::GRPC::Stub.new(AnyCable.config.rpc_host, :this_channel_is_insecure)
      end
    # ignore failed connections
    # (happens when using grpc_kit and launching server after setting up a service)
  rescue Errno::ECONNREFUSED
  end

  let(:service) do
    @service || begin
      if GRPC_KIT
        sock = TCPSocket.new(*AnyCable.config.rpc_host.split(":"))
        AnyCable::GRPC::Stub.new(sock)
      else
        AnyCable::GRPC::Stub.new(AnyCable.config.rpc_host, :this_channel_is_insecure)
      end
    end
  end
end
