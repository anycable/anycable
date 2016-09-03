# frozen_string_literal: true
shared_context "rpc stub" do
  before(:all) do
    @service = Anycable::RPC::Stub.new(Anycable.config.rpc_host, :this_channel_is_insecure)
  end

  let(:service) { @service }
end
