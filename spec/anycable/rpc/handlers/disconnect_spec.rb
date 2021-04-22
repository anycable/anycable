# frozen_string_literal: true

require "spec_helper"

describe AnyCable::RPC::Handlers::Disconnect do
  include_context "rpc_command"

  let(:user) { "disco" }
  let(:url) { "http://example.io/cable_lite?token=123" }
  let(:subscriptions) { %w[a b] }
  let(:headers) { {"Cookie" => "username=jack;"} }

  let(:request) do
    AnyCable::DisconnectRequest.new(
      identifiers: identifiers.to_json,
      subscriptions: subscriptions,
      env: env
    )
  end

  let(:log) { AnyCable::TestFactory.events_log }

  let(:handler) { AnyCable::RPC::Handler.new }

  subject { handler.disconnect(request) }

  it "invokes #disconnect method with correct data" do
    expect { subject }.to change { log.size }.by(1)

    expect(log.last[:data]).to eq(name: "disco", path: "/cable_lite", subscriptions: %w[a b])
  end

  context "when exception" do
    let(:url) { "http://example.io/cable_lite?raise=sudden_disconnect_error" }

    it "raises an exception", :aggregate_failures do
      expect { subject }.to raise_error("sudden_disconnect_error")
    end
  end
end
