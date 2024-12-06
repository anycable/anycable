# frozen_string_literal: true

require "spec_helper"

describe "WebSocket RPC" do
  include_context "rpc_command"

  before(:all) do
    @mock_server = MockWebSocketServer.new(token: "secret")
    @mock_server.start
  end

  after(:all) { @mock_server.stop }

  before { WebMock.disable! }
  after { WebMock.enable! }

  let(:mock_server) { @mock_server }

  let(:server_params) { {token: "secret"} }
  let(:server) { AnyCable::WSRPC::Server.new(url: @mock_server.ws_endpoint.url.to_s, **server_params) }

  subject(:client) do
    @client = server.build_client
  end

  def connect
    client.open
    mock_server.ensure_has_connection
  end

  after { @client&.close }

  describe "CONNECT" do
    let(:headers) do
      {
        "Cookie" => "username=john;"
      }
    end

    let(:url) { "http://example.io/cable?token=123" }

    let(:call_id) { SecureRandom.hex(3) }
    let(:meta) { {} }

    let(:request) { AnyCable::ConnectionRequest.new(env: env) }

    subject(:result) do
      connect

      mock_server.send_message({type: "command", command: "connect", payload: request.to_json, call_id: call_id, meta: meta}.to_json)

      response = mock_server.read_message
      @response = JSON.parse(response, symbolize_names: true)

      # Use .new(JSON.parse) instead of .decode_json to match Go implementation
      AnyCable::ConnectionResponse.new(**JSON.parse(@response[:payload], symbolize_names: true))
    end

    let(:result_id) { @response.fetch(:call_id) }

    it "responds with success", :aggregate_failures do
      expect(subject).to be_success
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        "current_user" => "john",
        "path" => "/cable",
        "token" => "123"
      )
      expect(subject.transmissions.first).to eq JSON.dump({"type" => "welcome"})
      expect(result_id).to eq call_id
    end

    it "invokes Connect handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:connect, request, an_instance_of(Hash))
    end

    context "with meta headers" do
      let(:meta) do
        {
          "sid" => "20230629"
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

    let(:call_id) { SecureRandom.hex(3) }
    let(:meta) { {} }

    subject(:result) do
      connect

      mock_server.send_message({type: "command", command: "disconnect", payload: request.to_json, call_id: call_id, meta: meta}.to_json)

      response = mock_server.read_message
      @response = JSON.parse(response, symbolize_names: true)

      AnyCable::DisconnectResponse.new(**JSON.parse(@response[:payload], symbolize_names: true))
    end

    let(:result_id) { @response.fetch(:call_id) }

    it "invokes disconnect with correct data" do
      expect(subject).to be_success
      expect(result_id).to eq call_id
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

    let(:request) do
      AnyCable::CommandMessage.new(
        command: command,
        identifier: channel_id,
        env: env,
        connection_identifiers: identifiers.to_json,
        data: data.to_json
      )
    end

    let(:call_id) { SecureRandom.hex(3) }
    let(:meta) { {} }

    subject(:result) do
      connect

      mock_server.send_message({type: "command", command: "command", payload: request.to_json, call_id: call_id, meta: meta}.to_json)

      response = mock_server.read_message
      @response = JSON.parse(response, symbolize_names: true)

      AnyCable::CommandResponse.new(**JSON.parse(@response[:payload], symbolize_names: true))
    end

    let(:result_id) { @response.fetch(:call_id) }

    it "responds with result" do
      expect(subject).to be_success
      expect(subject.transmissions.size).to eq 1
      expect(subject.transmissions.first).to include({"result" => {"data" => 3}}.to_json)
      expect(result_id).to eq call_id
    end

    it "invokes Command handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:command, request, an_instance_of(Hash))
    end
  end

  context "client-server interaction" do
    let(:messages) { Queue.new }

    def read_message
      messages.pop(timeout: 1)
    end

    subject(:client) do
      @client = server.build_client do |_, msg|
        messages << JSON.parse(msg, symbolize_names: true)
      end
    end

    describe "authentication" do
      it "missing token" do
        server_params.delete(:token)

        connect

        expect(read_message).to eq({type: "disconnect", reason: "unauthorized", reconnect: false})
        expect(client).to be_closed
      end

      it "invalid token" do
        server_params[:token] = "invalid"

        connect

        expect(read_message).to eq({type: "disconnect", reason: "unauthorized", reconnect: false})
        expect(client).to be_closed
      end

      it "valid token" do
        connect

        expect(read_message).to eq({type: "connect"})
        expect(client).to be_open
      end
    end

    context "reconnection" do
      specify do
        connect

        expect(read_message).to eq({type: "connect"})
        expect(client).to be_open

        mock_server.stop
        expect(read_message).to eq({type: "disconnect", reason: "server_restart", reconnect: true})

        sleep 2

        mock_server.start
        mock_server.ensure_has_connection

        expect(read_message).to eq({type: "connect"})
        expect(client).to be_open
      end
    end
  end
end
