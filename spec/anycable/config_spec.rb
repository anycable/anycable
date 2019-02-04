# frozen_string_literal: true

require "spec_helper"

describe "AnyCable::Config" do
  let(:described_class) { AnyCable::Config }
  subject(:config) { AnyCable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "0.0.0.0:50123"
    expect(config.rpc_host).not_to be_a(AnyCable::Config::DefaultHostWrapper)
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
        { "host" => "redis-1-1", "port" => 26_379 },
        { "host" => "redis-1-2", "port" => 26_380 }
      ]
    end

    context "with sentinel" do
      before do
        config.redis_sentinels = sentinel_config
      end

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2",
          sentinels: [{ "host" => "redis-1-1", "port" => 26_379 }, { "host" => "redis-1-2", "port" => 26_380 }]
        )
      end
    end

    context "when REDIS_SENTINEL_HOSTS" do
      around { |ex| with_env("ANYCABLE_REDIS_SENTINELS" => "redis-1-1:26379,redis-1-2:26380", &ex) }

      subject(:config) { described_class.new }

      specify do
        expect(subject.to_redis_params).to eq(
          url: "redis://localhost:6379/2",
          sentinels: [{ "host" => "redis-1-1", "port" => 26_379 }, { "host" => "redis-1-2", "port" => 26_380 }]
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
  end

  describe "defaults" do
    subject(:config) { described_class.new }

    # Initialize config with plain defaults
    around do |ex|
      old_conf, ENV["ANYCABLE_CONF"] = ENV["ANYCABLE_CONF"], nil
      ex.run
      ENV["ANYCABLE_CONF"] = old_conf
    end

    describe "#rpc_host" do
      # FIXME: remove in future
      if Gem::Version.new(AnyCable::VERSION) < Gem::Version.new("0.7")
        specify { expect(config.rpc_host).to be_a(AnyCable::Config::DefaultHostWrapper) }
        specify { expect(config.rpc_host).to eq("[::]:50051") }
      else
        specify { expect(config.rpc_host).to eq("127.0.0.1:50051") }
      end
    end
  end
end
