# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/redisx"

describe AnyCable::BroadcastAdapters::Redisx do
  let(:redis_conn) { double("redis_conn") }

  before do
    config.redis_url = "redis://redis-1:6478"
    config.redis_channel = "_test_"
    config.redis_sentinels = nil

    allow(::Redis).to receive(:new) { redis_conn }
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  describe "#announce" do
    around do |ex|
      old_logger = AnyCable.logger
      AnyCable.remove_instance_variable(:@logger)
      ex.run
      AnyCable.logger = old_logger
      AnyCable.config.reload
    end

    specify do
      expect { described_class.new.announce! }.to output(/Broadcasting Redis stream: _test_/).to_stdout_from_any_process
    end
  end

  describe "#broadcast" do
    it "add new entry to stream" do
      allow(redis_conn).to receive(:xadd)

      adapter = described_class.new
      adapter.broadcast("notification", "hello!")

      expect(redis_conn).to have_received(:xadd).with(
        "_test_",
        {payload: {stream: "notification", data: "hello!"}.to_json}
      )
    end
  end
end
