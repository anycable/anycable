# frozen_string_literal: true

require "spec_helper"

describe AnyCable::JWT do
  before do
    config.secret = "s3Krit"
  end

  after { config.reload }

  let(:config) { AnyCable.config }

  let(:payload) { {user_id: 26, tenant: "any"} }

  it "encodes and decodes payload" do
    token = described_class.encode(payload)

    expect(described_class.decode(token)).to eq(payload)
  end

  it "re-raises decode error" do
    token = described_class.encode(payload).dup

    token[0] = token[0].succ

    expect { described_class.decode(token) }.to raise_error(AnyCable::JWT::DecodeError)
  end

  it "raise expire error when expired" do
    token = described_class.encode(payload, expires_at: Time.now.to_i - 60)

    expect { described_class.decode(token) }.to raise_error(AnyCable::JWT::ExpiredSignature)
  end

  context "with existing token" do
    let(:token) { "eyJhbGciOiJIUzI1NiJ9.eyJleHQiOiJ7XCJ1c2VyXCI6XCJ2b3ZhXCIsXCJmZWF0dXJlc1wiOltcInJhaWxzXCIsXCJzdHJlYW1zXCJdfSJ9.5hJpXpC9deAFyfrCbOXpwJ55wm1RwyXRe2trtyVpqK4" }

    specify do
      expect(described_class.decode(token, secret_key: "qwerty"))
        .to eq(user: "vova", features: ["rails", "streams"])
    end
  end
end
