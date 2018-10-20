# frozen_string_literal: true

require "spec_helper"

describe "disconnection", :with_grpc_server, :rpc_command do
  include_context "rpc stub"

  let(:user) { "disco" }
  let(:url) { "http://example.io/cable_lite?token=123" }
  let(:subscriptions) { %w[a b] }
  let(:headers) { { "Cookie" => "username=jack;" } }

  let(:request) do
    Anycable::DisconnectRequest.new(
      identifiers: conn_id.to_json,
      subscriptions: subscriptions,
      path: url,
      headers: headers
    )
  end

  let(:log) { Anycable::TestFactory.events_log }

  subject { service.disconnect(request) }

  describe "Connection#disconnect" do
    it "invokes #disconnect method with correct data" do
      expect { subject }.to change { log.size }.by(1)

      expect(log.last[:data]).to eq(name: "disco", path: "/cable_lite", subscriptions: %w[a b])
    end
  end
end
