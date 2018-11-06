# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/redis"

describe AnyCable::BroadcastAdapters::Redis do
  let(:redis_conn) { double("redis_conn") }

  before do
    config.redis_url = "redis://redis-1:6478"
    config.redis_channel = "_test_"
    config.redis_sentinels = nil

    allow(::Redis).to receive(:new) { redis_conn }
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  it "uses config options by default" do
    adapter = described_class.new
    expect(adapter.channel).to eq "_test_"
    expect(Redis).to have_received(:new)
      .with(url: "redis://redis-1:6478")
  end

  it "uses override config params" do
    adapter = described_class.new(url: "redis://local.redis:123", channel: "_zyx_")
    expect(adapter.channel).to eq "_zyx_"
    expect(Redis).to have_received(:new)
      .with(url: "redis://local.redis:123")
  end

  describe "#broadcast" do
    it "publish data to channel" do
      allow(redis_conn).to receive(:publish)

      adapter = described_class.new
      adapter.broadcast("notification", "hello!")

      expect(redis_conn).to have_received(:publish).with(
        "_test_",
        { stream: "notification", data: "hello!" }.to_json
      )
    end
  end
end
