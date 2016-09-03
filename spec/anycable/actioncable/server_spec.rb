# frozen_string_literal: true
require "spec_helper"

describe ActionCable::Server::Base do
  describe "#broadcast" do
    it "calls Anycable::PubSub#broadcast" do
      expect_any_instance_of(Anycable::PubSub).to receive(:broadcast)
        .with('test', { type: 'test' }.to_json).once
      ActionCable.server.broadcast 'test', type: 'test'
    end
  end
end
