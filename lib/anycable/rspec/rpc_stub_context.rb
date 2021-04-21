# frozen_string_literal: true

RSpec.shared_context "anycable:rpc:stub" do
  before(:all) do
    @service = AnyCable::GRPC::Stub.new(AnyCable.config.rpc_host, :this_channel_is_insecure)
  end

  let(:service) { @service }

  let(:url) { "ws://example.anycable.com/cable" }
  let(:headers) { {} }
  let(:env) { AnyCable::Env.new(url: url, headers: headers) }
end
