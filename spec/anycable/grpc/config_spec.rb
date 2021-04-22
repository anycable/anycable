# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Config do
  subject(:config) { AnyCable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "0.0.0.0:50123"
    expect(config.log_grpc).to eq false
  end

  context "when DEBUG is set" do
    around { |ex| with_env("ANYCABLE_DEBUG" => "1", &ex) }

    subject(:config) { described_class.new }

    it "sets log_grpc to true" do
      expect(config.log_grpc).to eq true
    end
  end

  describe "defaults" do
    subject(:config) { described_class.new }

    around { |ex| with_env("ANYCABLE_CONF" => nil, &ex) }

    describe "#rpc_host" do
      specify { expect(config.rpc_host).to eq("127.0.0.1:50051") }
    end
  end
end
