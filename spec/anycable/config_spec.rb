# frozen_string_literal: true

require "spec_helper"

describe "AnyCable::Config" do
  let(:described_class) do
    require "anycable/config"
    AnyCable::Config
  end

  subject(:config) { AnyCable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "0.0.0.0:50123"
    expect(config.redis_channel).to eq "__anycable__"
    expect(config.log_level).to eq :info
    expect(config.log_grpc).to eq false
  end

  context "when DEBUG is set" do
    around { |ex| with_env("ANYCABLE_DEBUG" => "1", &ex) }

    subject(:config) { described_class.new }

    it "sets log to debug and log_grpc to true" do
      expect(config.log_level).to eq :debug
      expect(config.log_grpc).to eq true
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
  end

  describe "defaults" do
    subject(:config) { described_class.new }

    around { |ex| with_env("ANYCABLE_CONF" => nil, &ex) }

    describe "#rpc_host" do
      specify { expect(config.rpc_host).to eq("127.0.0.1:50051") }
    end
  end
end
