# frozen_string_literal: true

require "spec_helper"

describe "client connection", :with_grpc_server do
  include_context "rpc stub"

  subject { service.connect(request) }

  context "no cookies" do
    let(:request) { AnyCable::ConnectionRequest.new }

    it "responds with exception if no cookies" do
      expect(subject.status).to eq :FAILURE
    end
  end

  context "when exception" do
    let(:request) do
      AnyCable::ConnectionRequest.new(
        path: "http://example.io/cable?raise=sudden_connect_error"
      )
    end

    it "responds with ERROR", :aggregate_failures do
      expect(subject.status).to eq :ERROR
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

  context "with cookies and path info" do
    let(:request) do
      AnyCable::ConnectionRequest.new(
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
      AnyCable::ConnectionRequest.new(
        headers: {
          "cookie" => "username=john;",
          "x-api-token" => "abc123"
        },
        path: "http://example.io/cable"
      )
    end

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject.status).to eq :SUCCESS
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "token" => "abc123"
      )
      expect(subject.transmissions.first).to eq JSON.dump("type" => "welcome")
    end
  end

  context "with remote ip from headers" do
    let(:request) do
      AnyCable::ConnectionRequest.new(
        headers: {
          "cookie" => "username=john;",
          "REMOTE_ADDR" => "1.2.3.4"
        },
        path: "http://example.io/cable"
      )
    end

    it "sets ip and cleans synthetic header", :aggregate_failures do
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "ip" => "1.2.3.4",
        "remote_addr" => nil
      )
    end
  end

  describe "env building" do
    let(:request) do
      AnyCable::ConnectionRequest.new(
        headers: {
          "Cookie" => "username=john;"
        },
        path: "https://example.io/cable?token=123"
      )
    end

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
