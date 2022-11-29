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
        expect(subject.to_redis_params).to eq(url: "rediss://localhost:6379/3")
      end
    end

    context "with TLS and server certificate verification disabled" do
      subject(:config) { described_class.new }

      around { |ex| with_env("ANYCABLE_REDIS_URL" => "rediss://localhost:6379/3", "ANYCABLE_REDIS_SENTINELS" => "", "ANYCABLE_REDIS_TLS_VERIFY" => "no", &ex) }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "rediss://localhost:6379/3",
          ssl_params: {verify_mode: OpenSSL::SSL::VERIFY_NONE}
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
end
