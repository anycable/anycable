# frozen_string_literal: true

require "spec_helper"

describe "client connection", :with_grpc_server do
  include_context "rpc stub"

  subject { service.connect(request) }

  context "no cookies" do
    let(:request) { Anycable::ConnectionRequest.new }

    it "responds with exception if no cookies" do
      expect(subject.status).to eq :FAILURE
    end
  end

  context "with cookies and path info" do
    let(:request) do
      Anycable::ConnectionRequest.new(
        headers: {
          "Cookie" => "username=john;"
        },
        path: "http://example.io/cable?token=123"
      )
    end

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject.status).to eq :SUCCESS
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "current_user" => "john",
        "path" => "/cable",
        "token" => "123"
      )
      expect(subject.transmissions.first).to eq JSON.dump("type" => "welcome")
    end
  end

  context "with arbitrary headers" do
    let(:request) do
      Anycable::ConnectionRequest.new(
        headers: {
          "cookie" => "username=john;",
          "x-api-token" => "abc123",
          "X-Forwarded-For" => "1.2.3.4"
        },
        path: "http://example.io/cable"
      )
    end

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject.status).to eq :SUCCESS
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "token" => "abc123",
        "remote_ip" => "1.2.3.4"
      )
      expect(subject.transmissions.first).to eq JSON.dump("type" => "welcome")
    end
  end
end
