# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/nats"

describe AnyCable::BroadcastAdapters::Nats do
  let(:nats_conn) { instance_double("NATS::Client") }

  before do
    config.nats_servers = "nats://nats-1:4222"
    config.nats_channel = "_test_"

    allow(::NATS::Client).to receive(:new) { nats_conn }
    allow(nats_conn).to receive(:connect)
    allow(nats_conn).to receive(:on_disconnect)
    allow(nats_conn).to receive(:on_reconnect)
    allow(nats_conn).to receive(:on_error)
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  it "uses config options by default" do
    adapter = described_class.new
    expect(adapter.channel).to eq "_test_"
    expect(nats_conn).to have_received(:connect)
      .with(nil, {servers: ["nats://nats-1:4222"], dont_randomize_servers: false})
  end

  it "uses override config params" do
    adapter = described_class.new(servers: ["nats://natsy:8222", "nats://nasty:2228"], dont_randomize_servers: true, channel: "_zyx_")
    expect(adapter.channel).to eq "_zyx_"
    expect(nats_conn).to have_received(:connect)
      .with(nil, {servers: ["nats://natsy:8222", "nats://nasty:2228"], dont_randomize_servers: true})
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
      expect { described_class.new.announce! }.to output(/Broadcasting NATS channel: _test_/).to_stdout_from_any_process
    end
  end

  describe "#broadcast" do
    it "publish stream data to channel" do
      allow(nats_conn).to receive(:publish)

      adapter = described_class.new
      adapter.broadcast("notification", "hello!")

      expect(nats_conn).to have_received(:publish).with(
        "_test_",
        {stream: "notification", data: "hello!"}.to_json
      )
    end
  end

  describe "#broadcast_command" do
    it "publish command data to channel" do
      allow(nats_conn).to receive(:publish)

      adapter = described_class.new
      adapter.broadcast_command("disconnect", identifier: "42")

      expect(nats_conn).to have_received(:publish).with(
        "_test_",
        {command: "disconnect", payload: {identifier: "42"}}.to_json
      )
    end
  end
end
