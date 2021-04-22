# frozen_string_literal: true

RSpec.shared_context "anycable:grpc:stub" do
  include_context "rpc_command"

  before(:all) do
    @service = AnyCable::GRPC::Stub.new(AnyCable.config.rpc_host, :this_channel_is_insecure)
  end

  let(:service) { @service }
end
