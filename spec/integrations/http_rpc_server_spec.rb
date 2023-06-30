# frozen_string_literal: true

require "spec_helper"

describe "HTTP RPC" do
  include_context "rpc_command"

  let(:server) { AnyCable::HTTRPC::Server.new }

  def post(path, request, headers = {})
    env = Rack::MockRequest.env_for(File.join("http://localhost:3000", path), method: "POST")
    env["rack.input"] = StringIO.new(request.to_json)

    headers.each { |k, v| env["HTTP_#{k.upcase.tr("-", "_")}"] = v }

    server.call(env).then do |response|
      response = Rack::Response.new(response[2], response[0], response[1])
      raise "Unexpected result: #{response.code} #{response}" unless response.ok?
      response.body.first
    end
  end

  describe "bad requests" do
    it "not a POST request" do
      env = Rack::MockRequest.env_for("http://localhost:3000/connect", method: "GET")
      expect(server.call(env)).to eq [404, {}, ["Not found"]]
    end

    it "missing body" do
      env = Rack::MockRequest.env_for("http://localhost:3000/connect", method: "POST")
      expect(server.call(env)).to eq [422, {}, ["Empty request body"]]
    end

    it "uknown command" do
      env = Rack::MockRequest.env_for("http://localhost:3000/unknown", method: "POST")
      env["rack.input"] = StringIO.new({}.to_json)

      expect(server.call(env)).to eq [404, {}, ["Not found"]]
    end
  end

  describe "authentication" do
    let(:server) { AnyCable::HTTRPC::Server.new(token: "secret") }

    it "missing token" do
      env = Rack::MockRequest.env_for("http://localhost:3000/connect", method: "POST")

      expect(server.call(env)).to eq [401, {}, ["Unauthorized"]]
    end

    it "invalid token" do
      env = Rack::MockRequest.env_for("http://localhost:3000/connect", method: "POST")
      env["HTTP_AUTHORIZATION"] = "Bearer not-a-secret"

      expect(server.call(env)).to eq [401, {}, ["Unauthorized"]]
    end

    it "valid token" do
      env = Rack::MockRequest.env_for("http://localhost:3000/connect", method: "POST")
      env["HTTP_AUTHORIZATION"] = "Bearer secret"

      # we passed authentication check
      expect(server.call(env)).to eq [422, {}, ["Empty request body"]]
    end
  end

  describe "CONNECT" do
    let(:headers) do
      {
        "Cookie" => "username=john;"
      }
    end
    let(:url) { "http://example.io/cable?token=123" }

    let(:request_headers) { {} }
    let(:request) { AnyCable::ConnectionRequest.new(env: env) }

    let(:response) { post "/connect", request, request_headers }

    subject(:result) do
      AnyCable::ConnectionResponse.decode_json(response)
    end

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
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:connect, request, an_instance_of(Hash))
    end

    context "with meta headers" do
      let(:request_headers) do
        {
          "X-AnyCable-Meta-Sid" => "20230629"
        }
      end

      it "passes meta headers to the handler" do
        allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

        expect(subject).to be_success
        expect(AnyCable.rpc_handler)
          .to have_received(:handle).with(:connect, request, {"sid" => "20230629"})
      end
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

  describe "DISCONNECT" do
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

    let(:response) { post "/disconnect", request }

    subject(:result) do
      AnyCable::DisconnectResponse.decode_json(response)
    end

    it "invokes #disconnect method with correct data" do
      expect { subject }.to change { log.size }.by(1)

      expect(log.last[:data]).to eq(name: "disco", path: "/cable_lite", subscriptions: %w[a b])
    end

    it "invokes Disconnect handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:disconnect, request, an_instance_of(Hash))
    end
  end

  describe "COMMAND" do
    let(:channel_id) { "echo" }
    let(:command) { "message" }
    let(:data) { {action: "echo", data: 3} }

    let(:response) { post "/command", request }

    subject(:result) { AnyCable::CommandResponse.decode_json(response) }

    it "responds with result" do
      expect(subject).to be_success
      expect(subject.transmissions.size).to eq 1
      expect(subject.transmissions.first).to include({"result" => {"data" => 3}}.to_json)
    end

    it "invokes Command handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:command, request, an_instance_of(Hash))
    end
  end
end
