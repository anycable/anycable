# frozen_string_literal: true

require "spec_helper"
require "net/http"

describe "health server", :with_grpc_server do
  before(:all) do
    @health_server = AnyCable::HealthServer.new(
      @server,
      port: 54_321
    )
    @health_server.start
  end

  after(:all) { @health_server.stop }

  context "when server is running" do
    before { allow(@server).to receive(:running?).and_return(true) }

    it "responds with 200" do
      res = Net::HTTP.get_response(URI("http://localhost:54321/health"))
      expect(res.code).to eq "200"
    end
  end

  context "when server is not running" do
    before { allow(@server).to receive(:running?).and_return(false) }

    it "responds with 200" do
      res = Net::HTTP.get_response(URI("http://localhost:54321/health"))
      expect(res.code).to eq "503"
    end
  end
end
