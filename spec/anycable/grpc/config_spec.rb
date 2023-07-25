# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Config do
  subject(:config) { AnyCable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "0.0.0.0:50123"
    expect(config.log_grpc).to eq false
  end

  context "when DEBUG is set" do
    around { |ex| with_env("ANYCABLE_DEBUG" => "1", &ex) }

    subject(:config) { described_class.new }

    it "sets log_grpc to true" do
      expect(config.log_grpc).to eq true
    end
  end

  describe "defaults" do
    subject(:config) { described_class.new }

    around { |ex| with_env("ANYCABLE_CONF" => nil, &ex) }

    describe "#rpc_host" do
      specify { expect(config.rpc_host).to eq("127.0.0.1:50051") }
    end
  end

  describe "#to_grpc_params" do
    around { |ex| with_env("ANYCABLE_RPC_SERVER_ARGS__LOADREPORTING" => "1", &ex) }

    it "returns normalized server args" do
      config = described_class.new("rpc_server_args" => {max_connection_age_ms: 10_000, "grpc.per_message_compression" => 1, "max_concurrent_streams" => 10}) # rubocop:disable Style/HashSyntax

      server_args = config.to_grpc_params[:server_args]

      expect(server_args).to eq({
        "grpc.loadreporting" => 1,
        "grpc.max_connection_age_ms" => 10_000,
        "grpc.per_message_compression" => 1,
        "grpc.max_concurrent_streams" => 10
      })
    end

    it "returns default hash if server_args is not a hash" do
      expect(described_class.new(rpc_server_args: "foo").to_grpc_params[:server_args])
        .to eq({"grpc.max_connection_age_ms" => 300_000})
    end

    it "returns correct config if server_args is empty but rpc_max_connection_age is set" do
      expect(described_class.new(rpc_server_args: {}, rpc_max_connection_age: 60).to_grpc_params[:server_args])
        .to eq({"grpc.max_connection_age_ms" => 60_000})
    end
  end

  describe "#tls_credentials" do
    subject :tls_credentials do
      described_class.new.tls_credentials
    end

    it "returns insecure config if rpc_tls_cert and rpc_tls_key are not set" do
      expect(tls_credentials).to be_empty
    end

    context "when rpc_tls_cert and rpc_tls_key are set" do
      let(:cert) { "BASE64ABRACADABRA1" }
      let(:pkey) { "BASE64ABRACADABRA2" }

      around do |ex|
        with_env("ANYCABLE_RPC_TLS_CERT" => cert, "ANYCABLE_RPC_TLS_KEY" => pkey, &ex)
      end

      it "returns TLS-enabled config" do
        expect(tls_credentials).to include(cert: cert, pkey: pkey)
      end
    end
  end
end
