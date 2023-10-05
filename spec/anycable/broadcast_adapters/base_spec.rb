# frozen_string_literal: true

require "spec_helper"

require "anycable/broadcast_adapters/base"

describe AnyCable::BroadcastAdapters::Base do
  let(:broadcaster_class) do
    Class.new(described_class) do
      attr_reader :sent

      def initialize
        @sent = []
      end

      def raw_broadcast(payload)
        @sent << payload
      end
    end
  end

  subject(:broadcaster) { broadcaster_class.new }

  describe "#batching" do
    it "aggregates broadcasts" do
      subject.broadcast("test", "a")

      expect(subject.sent).to eq([
        {stream: "test", data: "a"}.to_json
      ])

      subject.batching do
        subject.broadcast("test", "b")
        subject.broadcast("test:2", "foo")

        expect(subject.sent.size).to eq 1
      end

      expect(subject.sent.size).to eq 2

      expect(subject.sent.last).to eq(
        [
          {stream: "test", data: "b"},
          {stream: "test:2", data: "foo"}
        ].to_json
      )
    end

    it "supports nesting" do
      subject.batching do
        subject.broadcast("test", "a")
        subject.broadcast("test:2", "foo")

        expect(subject.sent.size).to eq 0

        subject.batching(false) do
          subject.broadcast("test", "b")

          subject.batching do
            subject.broadcast("test", "c")
          end
        end

        expect(subject.sent.size).to eq 1

        expect(subject.sent).to eq([
          {stream: "test", data: "b"}.to_json
        ])
      end

      expect(subject.sent.size).to eq 2

      expect(subject.sent.last).to eq(
        [
          {stream: "test", data: "a"},
          {stream: "test:2", data: "foo"},
          {stream: "test", data: "c"}
        ].to_json
      )
    end
  end
end
