# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Streams do
  before do
    config.secret = "s3Krit"
  end

  after { AnyCable.config.reload }

  let(:config) { AnyCable.config }

  describe "#signed" do
    it "returns signed stream name" do
      encoded, signature = described_class.signed("user_1").split("--")

      expect(Base64.urlsafe_decode64(encoded)).to eq(%("user_1"))
      expect(signature).to eq OpenSSL::HMAC.hexdigest("SHA256", "s3Krit", encoded)
    end

    context "when secret is not set" do
      before { config.secret = nil }

      it "raises error" do
        expect { described_class.signed("user_1") }.to raise_error(ArgumentError, /secret is missing/)
      end
    end

    context "when streams_secret is specified" do
      before { config.streams_secret = "qwerty" }

      it "returns signed stream name" do
        encoded, signature = described_class.signed("user_1").split("--")

        expect(signature).to eq OpenSSL::HMAC.hexdigest("SHA256", "qwerty", encoded)
      end
    end
  end

  describe "#verified" do
    # Turbo.signed_stream_verifier_key = 's3Krit'
    # Turbo::StreamsChannel.signed_stream_name([:chat, "2021"])
    let(:signed_stream_name) { "ImNoYXQ6MjAyMSI=--f9ee45dbccb1da04d8ceb99cc820207804370ba0d06b46fc3b8b373af1315628" }

    it "verifies signed stream name" do
      expect(described_class.verified(signed_stream_name)).to eq "chat:2021"
    end

    it "round-trip" do
      expect(described_class.verified(described_class.signed("chat:2021"))).to eq "chat:2021"
    end

    context "when signature is invalid" do
      let(:signed_stream_name) { "ImNoYXQ6MjAyMSI=--f9ee45dbccb1da04d8ceb99cc820207804370ba0d06b46fc3b8b373af1315629" }

      it "returns nil" do
        expect(described_class.verified(signed_stream_name)).to be_nil
      end
    end
  end
end
