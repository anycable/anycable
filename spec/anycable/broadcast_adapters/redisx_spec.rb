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
