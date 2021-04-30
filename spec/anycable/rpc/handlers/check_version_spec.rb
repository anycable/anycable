# frozen_string_literal: true

require "spec_helper"

describe "version check spec" do
  include_context "rpc_command"

  let(:headers) do
    {
      "Cookie" => "username=john;"
    }
  end
  let(:middleware) do
    AnyCable::MiddlewareChain.new.tap do |chain|
      chain.use(AnyCable::Middlewares::CheckVersion.new("test-v1"))
    end
  end

  let(:request) { AnyCable::ConnectionRequest.new(env: env) }
  let(:meta) { {} }
  let(:handler) { AnyCable::RPC::Handler.new(middleware: middleware) }

  subject { handler.handle(:connect, request, meta) }

  it "passes with a single matching version in meta" do
    meta["protov"] = "test-v1"
    expect(subject).to be_success
  end

  it "passes with multiple versions including matching" do
    meta["protov"] = "test-v0,test-v1"
    expect(subject).to be_success
  end

  it "fails without matching version" do
    meta["protov"] = "test-v0,test-v01"
    expect { subject }.to raise_error(
      %r{Client supported versions: test-v0,test-v01}
    )
  end

  it "fails without metadata" do
    expect { subject }.to raise_error(
      %r{Client supported versions: unknown}
    )
  end
end
