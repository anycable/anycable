# frozen_string_literal: true

require 'spec_helper'

describe Anycable::PubSub do
  let(:config) { Anycable.config }
  subject { described_class.new }

  after { Anycable.config.reload }

  context "with sentinel" do
    it do
      expect(Redis).to receive(:new).with(url: config.redis_url, sentinels: config.redis_sentinels)
      subject
    end
  end

  context "without sentinel" do
    before do
      Anycable.config.redis_sentinels = []
    end

    it do
      expect(Redis).to receive(:new).with(url: config.redis_url)
      subject
    end
  end
end
