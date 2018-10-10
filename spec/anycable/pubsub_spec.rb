# frozen_string_literal: true

require 'spec_helper'

describe Anycable::PubSub do
  let(:config) { RedisConfig.new }
  subject { described_class.new(config) }

  context "with sentinel" do
    let(:config) { Anycable::RedisConfig.new(overrides: { url: "localhost:4310/0", sentinels: [{ host: "redis-1", port: 26_379 }, { host: "redis-12", port: 26_380 }] }) }

    it do
      expect(Redis).to receive(:new).with(url: "localhost:4310/0", sentinels: [{ host: "redis-1", port: 26_379 }, { host: "redis-12", port: 26_380 }])
      subject
    end
  end

  context "without sentinel" do
    let(:config) { Anycable::RedisConfig.new(overrides: { url: "localhost:4310/0", sentinels: [] }) }

    it do
      expect(Redis).to receive(:new).with(url: "localhost:4310/0")
      subject
    end
  end
end
