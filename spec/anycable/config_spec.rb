# frozen_string_literal: true

require 'spec_helper'

describe Anycable::Config do
  subject(:config) { Anycable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "localhost:50123"
    expect(config.redis_url).to eq "redis://localhost:6379/2"
    # default value
    expect(config.redis_channel).to eq "__anycable__"
    expect(config.log_level).to eq :info
    expect(config.log_grpc).to eq false
  end

  context "when DEBUG is set" do
    around { |ex| with_env('ANYCABLE_DEBUG' => '1', &ex) }

    subject(:config) { described_class.new }

    it "sets log to debug and log_grpc to true" do
      expect(config.log_level).to eq :debug
      expect(config.log_grpc).to eq true
    end
  end
end
