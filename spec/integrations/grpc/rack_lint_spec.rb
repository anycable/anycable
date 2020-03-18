# frozen_string_literal: true

require "spec_helper"

# Only lint rack env in this spec to avoid
# failing all the specs if it's invalid
describe "rack lint" do
  include_context "anycable:rpc:server"
  include_context "rpc_command"

  let(:headers) do
    {
      "Cookie" => "username=john;"
    }
  end
  let(:url) { "ws://example.com/cable?rack=lint" }
  let(:request) { AnyCable::ConnectionRequest.new(env: env) }

  subject { service.connect(request) }

  specify do
    expect(subject).to be_success
  end
end
