# frozen_string_literal: true

require "spec_helper"

describe "client connection" do
  include_context "anycable:rpc:server"
  include_context "rpc_command"

  let(:headers) do
    {
      "Cookie" => "username=john;"
    }
  end
  let(:url) { "http://example.io/cable?token=123" }

  let(:request) { AnyCable::ConnectionRequest.new(env: env) }

  subject { service.connect(request) }

  it "responds with success", :aggregate_failures do
    expect(subject).to be_success
    identifiers = JSON.parse(subject.identifiers)
    expect(identifiers).to include(
      "current_user" => "john",
      "path" => "/cable",
      "token" => "123"
    )
    expect(subject.transmissions.first).to eq JSON.dump("type" => "welcome")
  end

  it "invokes Connect handler" do
    allow(AnyCable::RPC::Handlers::Connect).to receive(:call).and_call_original

    expect(subject).to be_success
    expect(AnyCable::RPC::Handlers::Connect).to have_received(:call).with(request)
  end

  context "when exception" do
    let(:url) { "http://example.io/cable?raise=sudden_connect_error" }

    it "responds with ERROR", :aggregate_failures do
      expect(subject).to be_error
      expect(subject.error_msg).to eq("sudden_connect_error")
    end

    it "notifies exception handler" do
      subject

      expect(TestExHandler.last_error).to have_attributes(
        exception: have_attributes(message: "sudden_connect_error"),
        method: "connect",
        message: request.to_h
      )
    end
  end
end
