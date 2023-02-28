# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Config do
  subject(:config) { AnyCable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.redis_channel).to eq "__anycable__"
    expect(config.log_level).to eq "info"
  end

  context "when DEBUG is set" do
    around { |ex| with_env("ANYCABLE_DEBUG" => "1", &ex) }

    subject(:config) { described_class.new }

    it "sets log to debug" do
      expect(config.log_level).to eq "debug"
    end
  end

  describe "#to_redis_params" do
    let(:sentinel_config) do
      [
        {"host" => "redis-1-1", "port" => 26_379},
        {"host" => "redis-1-2", "port" => 26_380, "password" => "secret"}
      ]
    end

    context "with sentinel" do
      before do
        config.redis_sentinels = sentinel_config
      end

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2",
          sentinels: [{host: "redis-1-1", port: 26_379}, {host: "redis-1-2", port: 26_380, password: "secret"}]
        )
      end
    end

    context "with ANYCABLE_REDIS_SENTINELS" do
      around { |ex| with_env("ANYCABLE_REDIS_SENTINELS" => "redis-1-1:26379,:secret@redis-1-2:26380", &ex) }

      subject(:config) { described_class.new }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2",
          sentinels: [{host: "redis-1-1", port: 26_379}, {host: "redis-1-2", port: 26_380, password: "secret"}]
        )
      end
    end

    context "when ANYCABLE_REDIS_SENTINELS contains only one sentinel" do
      around { |ex| with_env("ANYCABLE_REDIS_SENTINELS" => "redis-1-1:26379", &ex) }

      subject(:config) { described_class.new }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2",
          sentinels: [{host: "redis-1-1", port: 26_379}]
        )
      end
    end

    context "without sentinel" do
      before do
        config.redis_sentinels = []
      end

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2"
        )
      end
    end

    context "with TLS" do
      subject(:config) { described_class.new }

      around { |ex| with_env("ANYCABLE_REDIS_URL" => "rediss://localhost:6379/3", "ANYCABLE_REDIS_SENTINELS" => "", &ex) }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "rediss://localhost:6379/3",
          ssl_params: {verify_mode: OpenSSL::SSL::VERIFY_NONE}
        )
      end
    end

    context "with TLS and server certificate verification enabled" do
      subject(:config) { described_class.new }

      around { |ex| with_env("ANYCABLE_REDIS_URL" => "rediss://localhost:6379/3", "ANYCABLE_REDIS_TLS_VERIFY" => "1", "ANYCABLE_REDIS_SENTINELS" => "", &ex) }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "rediss://localhost:6379/3"
        )
      end
    end
  end

  describe "#to_nats_params" do
    context "with multiple servers" do
      before do
        config.nats_servers = ["nats://one:4242", "nats://two:4242"]
      end

      specify do
        expect(subject.to_nats_params).to eq(
          servers: ["nats://one:4242", "nats://two:4242"],
          dont_randomize_servers: false
        )
      end
    end

    context "with ANYCABLE_NATS_SERVERS and ANYCABLE_NATS_DONT_RANDOMIZE_SERVERS" do
      around do |ex|
        with_env({
          "ANYCABLE_NATS_SERVERS" => "nats://uno:42, nats://duo:43",
          "ANYCABLE_NATS_DONT_RANDOMIZE_SERVERS" => "1"
        }, &ex)
      end

      subject(:config) { described_class.new }

      specify do
        expect(subject.to_nats_params).to eq(
          servers: ["nats://uno:42", "nats://duo:43"],
          dont_randomize_servers: true
        )
      end
    end
  end

  describe "defaults" do
    subject(:config) { described_class.new }

    around { |ex| with_env("ANYCABLE_CONF" => nil, &ex) }

    describe "#redis_url" do
      specify { expect(config.redis_url).to eq("redis://localhost:6379/5") }
    end
  end

  describe "presets", grpc: true do
    subject(:config) { described_class.new }

    around { |ex| with_env("ANYCABLE_CONF" => nil, &ex) }

    specify "no presets detected" do
      expect(config.presets).to eq([])
    end

    context "with Fly env" do
      around do |ex|
        with_env(
          "FLY_APP_NAME" => "real-timer",
          "FLY_ALLOC_ID" => "431",
          "FLY_REGION" => "syd",
          &ex
        )
      end

      specify do
        expect(config.rpc_host).to eq("0.0.0.0:50051")
      end

      context "with WS app name" do
        around { |ex| with_env("ANYCABLE_FLY_WS_APP_NAME" => "kabel", &ex) }

        specify do
          expect(config.rpc_host).to eq("0.0.0.0:50051")
          expect(config.http_broadcast_url).to eq("http://syd.kabel.internal:8090/_broadcast")
          expect(config.nats_servers).to eq(["nats://syd.kabel.internal:4222"])
        end

        context "with explicit settings" do
          around do |ex|
            with_env(
              "ANYCABLE_NATS_SERVERS" => "nats://some.other.nats:4321",
              "ANYCABLE_RPC_HOST" => "0.0.0.0:50061",
              &ex
            )
          end

          specify do
            expect(config.rpc_host).to eq("0.0.0.0:50061")
            expect(config.nats_servers).to eq(["nats://some.other.nats:4321"])
            expect(config.http_broadcast_url).to eq("http://syd.kabel.internal:8090/_broadcast")
          end
        end
      end

      context "when none provided" do
        around { |ex| with_env("ANYCABLE_PRESETS" => "none", &ex) }

        specify do
          expect(config.rpc_host).to eq("127.0.0.1:50051")
        end
      end
    end
  end
end
