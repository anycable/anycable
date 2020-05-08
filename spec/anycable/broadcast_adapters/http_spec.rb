# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/http"

describe AnyCable::BroadcastAdapters::Http do
  before do
    config.http_broadcast_url = "http://ws.example.com/_broadcast"
    config.http_broadcast_secret = "my-secret"
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  it "uses config options by default" do
    adapter = described_class.new
    expect(adapter.url).to eq "http://ws.example.com/_broadcast"
    expect(adapter.headers).to eq("Authorization" => "Bearer my-secret")
  end

  it "uses override config params" do
    adapter = described_class.new(url: "http://example.com/ws/_broadcast", secret: "tshhh")
    expect(adapter.url).to eq "http://example.com/ws/_broadcast"
    expect(adapter.headers).to eq("Authorization" => "Bearer tshhh")
  end

  describe "#broadcast" do
    let(:adapter) { described_class.new(url: "http://ws.example.com:8090/_broadcast") }
    subject { adapter }

    it "publish data to channel" do
      stub_request(:post, "http://ws.example.com:8090/_broadcast").to_return(status: 201)

      subject.broadcast("notification", "hello!")

      # make sure thread has processed messages
      subject.shutdown

      expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
        .with(body: {stream: "notification", data: "hello!"}.to_json))
        .to have_been_made.once
    end

    it "print debug message if response is not 201" do
      allow(AnyCable.logger).to receive(:debug)
      stub_request(:post, "http://ws.example.com:8090/_broadcast").to_return(status: 403)

      subject.broadcast("notification", "hello!")

      # make sure thread has processed messages
      subject.shutdown

      expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
        .with(body: {stream: "notification", data: "hello!"}.to_json))
        .to have_been_made.once

      expect(AnyCable.logger).to have_received(:debug).with(/Broadcast request responded with unexpected status: 403/)
    end

    context "with authorization" do
      let(:adapter) { described_class.new(url: "http://ws.example.com:8090/_broadcast", secret: "any-secret") }

      it "adds Authorization header" do
        stub_request(:post, "http://ws.example.com:8090/_broadcast").to_return(status: 201)

        subject.broadcast("notification", "hello!")

        # make sure thread has processed messages
        subject.shutdown

        expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
          .with(
            body: {stream: "notification", data: "hello!"}.to_json,
            headers: {"Authorization" => "Bearer any-secret"}
          )
        ).to have_been_made.once
      end
    end

    context "retries" do
      subject { adapter.broadcast("notification", "hello!") }

      before do
        allow(adapter).to receive(:sleep)
        allow(AnyCable.logger).to receive(:error)
      end

      it "handles TimeoutError and retries" do
        stub_request(:post, "http://ws.example.com:8090/_broadcast")
          .to_raise(Timeout::Error).then
          .to_return(status: 201)

        subject

        # make sure thread has processed messages
        adapter.shutdown

        expect(AnyCable.logger).not_to have_received(:error)

        expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
          .with(body: {stream: "notification", data: "hello!"}.to_json))
          .to have_been_made.twice
      end

      it "retires only 2 times" do
        stub_request(:post, "http://ws.example.com:8090/_broadcast")
          .to_raise(Timeout::Error).then
          .to_raise(Net::ProtocolError).then
          .to_raise(Timeout::Error)

        subject

        # make sure thread has processed messages
        adapter.shutdown

        expect(AnyCable.logger).to have_received(:error).with(/Broadcast request failed/)
      end

      context "thread keepalive" do
        it "re-creates a thread of exception" do
          stub_request(:post, "http://ws.example.com:8090/_broadcast")
            .to_raise(RuntimeError).then
            .to_return(status: 201)

          # silent thread exceptions
          adapter.send(:ensure_thread_is_alive)
          adapter.send(:thread).report_on_exception = false

          subject

          # make sure thread has processed messages
          adapter.shutdown
          # remove stop frame which haven't been processed by the dead thread
          expect(adapter.send(:queue).pop).to eq :stop

          expect(AnyCable.logger).to have_received(:error).with(/Broadcasting thread exited with exception/)

          adapter.broadcast("notification", "hi again!")

          # make sure thread has processed messages
          adapter.shutdown

          expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
            .with(body: {stream: "notification", data: "hello!"}.to_json))
            .to have_been_made

          expect(a_request(:post, "http://ws.example.com:8090/_broadcast")
            .with(body: {stream: "notification", data: "hi again!"}.to_json))
            .to have_been_made
        end
      end
    end
  end
end
