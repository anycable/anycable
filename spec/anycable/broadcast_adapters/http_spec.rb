# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/http"

describe AnyCable::BroadcastAdapters::Http do
  before do
    config.http_broadcast_url = "http://ws.example.com/_broadcast"
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  it "uses config options by default" do
    adapter = described_class.new
    expect(adapter.url).to eq "http://ws.example.com/_broadcast"
  end

  it "uses override config params" do
    adapter = described_class.new(url: "http://example.com/ws/_broadcast")
    expect(adapter.url).to eq "http://example.com/ws/_broadcast"
  end

  describe "#broadcast" do
    subject { described_class.new }

    it "publish data to channel" do
      stub_request(:post, "http://ws.example.com/_broadcast").to_return(status: 201)

      subject.broadcast("notification", "hello!")

      expect(a_request(:post, "http://ws.example.com/_broadcast")
        .with(body: {stream: "notification", data: "hello!"}.to_json))
        .to have_been_made.once
    end

    context "retries" do
      let(:adapter) { described_class.new }
      subject { adapter.broadcast("notification", "hello!") }

      before do
        allow(adapter).to receive(:sleep)
      end

      it "handles TimeoutError and retries" do
        stub_request(:post, "http://ws.example.com/_broadcast")
          .to_raise(Timeout::Error).then
          .to_return(status: 201)

        expect { subject }.not_to raise_error

        expect(a_request(:post, "http://ws.example.com/_broadcast")
          .with(body: {stream: "notification", data: "hello!"}.to_json))
          .to have_been_made.twice
      end

      it "retires only 2 times" do
        stub_request(:post, "http://ws.example.com/_broadcast")
          .to_raise(Timeout::Error).then
          .to_raise(Net::HTTPBadResponse).then
          .to_raise(Timeout::Error)

        expect { subject }.to raise_error(Timeout::Error)
      end
    end
  end
end
