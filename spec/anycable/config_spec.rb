# frozen_string_literal: true

require 'spec_helper'

describe Anycable::Config do
  subject(:config) { Anycable.config }

  it "loads config vars from anycable.yml", :aggregate_failures do
    expect(config.rpc_host).to eq "localhost:50123"
    expect(config.redis_url).to eq "redis://localhost:6379/2"
    # default value
    expect(config.redis_channel).to eq "anycable"
  end
end
