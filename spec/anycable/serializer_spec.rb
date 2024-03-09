# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Serializer do
  describe "#deserialize" do
    specify "primitive value", :aggregate_failures do
      expect(described_class.deserialize(1)).to eq 1
      expect(described_class.deserialize("1")).to eq "1"
      expect(described_class.deserialize(true)).to eq true
    end

    specify "hash value" do
      expect(described_class.deserialize({"a" => "b"})).to eq({"a" => "b"})
    end
  end

  describe "#serialize" do
    specify "primitive value" do
      expect(described_class.serialize(1)).to eq 1
      expect(described_class.serialize("1")).to eq "1"
      expect(described_class.serialize(true)).to eq true
    end

    specify "hash value" do
      expect(described_class.deserialize({"a" => "b"})).to eq({"a" => "b"})
    end
  end

  context "with object_serializer" do
    before do
      serializer = double("object_serializer")
      allow(serializer).to receive(:serialize) { |val| val.is_a?(user_class) ? "user:#{val.id}" : nil }
      allow(serializer).to receive(:deserialize) { |val| val.start_with?("user:") ? user_class.new(val.split(":").last.to_i) : nil }

      described_class.object_serializer = serializer
    end

    after { described_class.object_serializer = nil }

    let(:user_class) { Class.new(Struct.new(:id)) }

    let(:user) { user_class.new(26) }

    it "serializes and deserializes Global IDs" do
      expect(described_class.serialize(user)).to eq("user:26")
      expect(described_class.deserialize("user:26")).to eq(user)
    end

    it "works with Arrays and Hashes" do
      gid = "user:26"

      payload = {user: {model: user}, users: [user]}

      serialized = described_class.serialize(payload)

      expect(serialized).to eq({user: {model: gid}, users: [gid]})

      deserialized = described_class.deserialize(serialized)

      expect(deserialized).to eq(payload)
    end
  end
end
