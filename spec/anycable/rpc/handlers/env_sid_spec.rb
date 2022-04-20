# frozen_string_literal: true

require "spec_helper"

describe "env sid middleware" do
  include_context "rpc_command"

  let(:headers) do
    {
      "Cookie" => "username=john;"
    }
  end
  let(:middleware) do
    AnyCable::MiddlewareChain.new.tap do |chain|
      chain.use(AnyCable::Middlewares::EnvSid)
    end
  end

  let(:request) { AnyCable::ConnectionRequest.new(env: env) }
  let(:meta) { {} }
  let(:handler) { AnyCable::RPC::Handler.new(middleware: middleware) }

  subject { handler.handle(:connect, request, meta) }

  it "modifies request env in-place" do
    meta["sid"] = "test-123"
    expect(subject).to be_success
    expect(request.env.sid).to eq "test-123"
  end
end
