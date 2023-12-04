# frozen_string_literal: true

require "spec_helper"

describe AnyCable::RPC::Handlers::Connect do
  include_context "rpc_command"

  let(:handler) { AnyCable::RPC::Handler.new }

  let(:request) { AnyCable::ConnectionRequest.new(env: env) }

  subject { handler.connect(request) }

  context "no cookies" do
    it "responds with failure if no cookies" do
      expect(subject).to be_failure
    end

    it "returns disconnect message" do
      expect(subject).to be_failure
      expect(subject.transmissions).to eq(
        [JSON.dump({"type" => "disconnect", "reason" => "unauthorized"})]
      )
    end
  end

  context "when exception" do
    let(:url) { "http://example.io/cable?raise=sudden_connect_error" }

    it "raises an exception", :aggregate_failures do
      expect { subject }.to raise_error("sudden_connect_error")
    end
  end

  context "with cookies and path info" do
    let(:headers) do
      {
        "Cookie" => "username=john;"
      }
    end
    let(:url) { "http://example.io/cable?token=123" }

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject).to be_success
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "current_user" => "john",
        "path" => "/cable",
        "token" => "123"
      )
      expect(subject.transmissions.first).to eq JSON.dump({"type" => "welcome"})
    end
  end

  context "with arbitrary headers" do
    let(:headers) do
      {
        "cookie" => "username=john;",
        "x-api-token" => "abc123"
      }
    end
    let(:url) { "http://example.io/cable" }

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject).to be_success
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "token" => "abc123"
      )
      expect(subject.transmissions.first).to eq JSON.dump({"type" => "welcome"})
    end
  end

  context "with remote ip from headers" do
    let(:headers) do
      {
        "cookie" => "username=john;",
        "REMOTE_ADDR" => "1.2.3.4"
      }
    end
    let(:url) { "http://example.io/cable" }

    it "sets ip and cleans synthetic header", :aggregate_failures do
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "ip" => "1.2.3.4",
        "remote_addr" => nil
      )
    end
  end

  describe "Rack env building" do
    let(:headers) do
      {
        "Cookie" => "username=john;"
      }
    end
    let(:url) { "https://example.io/cable?token=123" }

    it "builds properly structured Rack-compatible env" do
      identifiers = JSON.parse(subject.identifiers)
      request = Rack::Request.new(identifiers["env"])

      expect(request).to have_attributes(
        request_method: "GET",
        script_name: "",
        scheme: "https",
        host: "example.io",
        port: 443,
        path: "/cable",
        query_string: "token=123",
        params: a_hash_including(
          "token" => "123"
        )
      )
    end
  end
end
