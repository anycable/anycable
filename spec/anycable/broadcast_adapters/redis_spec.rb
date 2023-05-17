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
      .with(url: "redis://redis-1:6478", driver: :ruby)
  end

  it "uses override config params" do
    adapter = described_class.new(url: "redis://local.redis:123", channel: "_zyx_")
    expect(adapter.channel).to eq "_zyx_"
    expect(Redis).to have_received(:new)
      .with(url: "redis://local.redis:123", driver: :ruby)
  end

  it "allows for other drivers" do
    described_class.new(driver: :memory)
    expect(Redis).to have_received(:new)
      .with(driver: :memory, url: anything)
  end

  describe "#announce" do
    around do |ex|
      old_logger = AnyCable.logger
      AnyCable.remove_instance_variable(:@logger)
      ex.run
      AnyCable.logger = old_logger
      AnyCable.config.reload
    end

    specify do
      expect { described_class.new.announce! }.to output(/Broadcasting Redis channel: _test_/).to_stdout_from_any_process
    end
  end

  describe "#broadcast" do
    it "publish stream data to channel" do
      allow(redis_conn).to receive(:publish)

      adapter = described_class.new
      adapter.broadcast("notification", "hello!")

      expect(redis_conn).to have_received(:publish).with(
        "_test_",
        {stream: "notification", data: "hello!"}.to_json
      )
    end
  end

  describe "#broadcast_command" do
    it "publish command data to channel" do
      allow(redis_conn).to receive(:publish)

      adapter = described_class.new
      adapter.broadcast_command("disconnect", identifier: "42")

      expect(redis_conn).to have_received(:publish).with(
        "_test_",
        {command: "disconnect", payload: {identifier: "42"}}.to_json
      )
    end
  end
end
