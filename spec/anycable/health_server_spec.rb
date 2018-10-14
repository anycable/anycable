# frozen_string_literal: true

require "spec_helper"
require "bg_helper"
require "net/http"

describe "health server" do
  context "when server is running" do
    before { allow_any_instance_of(Anycable::Server).to receive(:running?).and_return(true) }

    it "responds with 200" do
      res = Net::HTTP.get_response(URI("http://localhost:#{Anycable.config.http_health_port}/health"))
      expect(res.code).to eq "200"
    end
  end

  context "when server is not running" do
    before { allow_any_instance_of(Anycable::Server).to receive(:running?).and_return(false) }

    it "responds with 200" do
      res = Net::HTTP.get_response(URI("http://localhost:#{Anycable.config.http_health_port}/health"))
      expect(res.code).to eq "503"
    end
  end
end
