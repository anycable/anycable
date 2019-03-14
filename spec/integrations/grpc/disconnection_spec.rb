# frozen_string_literal: true

require "spec_helper"

describe "disconnection", :with_grpc_server, :rpc_command do
  include_context "rpc stub"

  let(:user) { "disco" }
  let(:url) { "http://example.io/cable_lite?token=123" }
  let(:subscriptions) { %w[a b] }
  let(:headers) { { "Cookie" => "username=jack;" } }

  let(:request) do
    AnyCable::DisconnectRequest.new(
      identifiers: conn_id.to_json,
      subscriptions: subscriptions,
      path: url,
      headers: headers
    )
  end

  let(:log) { AnyCable::TestFactory.events_log }

  subject { service.disconnect(request) }

  describe "Connection#disconnect" do
    it "invokes #disconnect method with correct data" do
      expect { subject }.to change { log.size }.by(1)

      expect(log.last[:data]).to eq(name: "disco", path: "/cable_lite", subscriptions: %w[a b])
    end
  end

  context "when exception" do
    let(:url) { "http://example.io/cable_lite?raise=sudden_disconnect_error" }

    it "responds with ERROR", :aggregate_failures do
      expect(subject.status).to eq :ERROR
      expect(subject.error_msg).to eq("sudden_disconnect_error")
    end

    it "notifies exception handler" do
      subject

      expect(TestExHandler.last_error).to have_attributes(
        exception: have_attributes(message: "sudden_disconnect_error"),
        method: "disconnect",
        message: request.to_h
      )
    end
  end
end
