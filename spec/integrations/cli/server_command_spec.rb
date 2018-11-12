# frozen_string_literal: true

require "spec_helper"
require "net/http"

describe "CLI server command", :cli do
  it "runs server and stops it on exit" do
    run_cli(
      "-r ../spec/dummies/app.rb " \
      "--rpc-host 0.0.0.0:50042 " \
      "--server-command 'ruby ../spec/dummies/server.rb'"
    ) do |cli|
      expect(cli).to have_output_line("WEBrick::HTTPServer#start:")
      res = Net::HTTP.get_response(URI("http://localhost:9021/"))
      expect(res.code).to eq "200"
    end

    sleep 0.5

    expect do
      Net::HTTP.get_response(URI("http://localhost:9021/"))
    end.to raise_error(Errno::ECONNREFUSED)
  end
end
