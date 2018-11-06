# frozen_string_literal: true

require "bundler/inline"

# This reproduction script is based on `anyt` gem
# (https://github.com/anycable/anyt).
#
# See more test examples here:
# https://github.com/anycable/anyt/tree/master/lib/anyt/tests

gemfile(true) do
  source "https://rubygems.org"

  gem "anyt"
end

require "anyt"
require "anyt/cli"
require "anycable-rails"

# WebSocket server url
ENV['ANYT_TARGET_URL'] ||= "ws://localhost:8080/cable"

# Command to launch WebSocket server.
# Comment this line if you want to run WebSocket server manually
ENV['ANYT_COMMAND'] ||= "anycable-go"

ActionCable.server.config.logger = Rails.logger = AnyCable.logger

# Test scenario
feature "issue_xyz" do
  # This block defines an anonymous channel to test against
  channel do
    # def subscribed
    #  stream_from "a"
    # end
    #
    # def perform(data)
    #  # ...
    # end
  end

  # You can use minitest/spec features here
  before do
    # `channel` contains identifier of the anonymous channel defined above
    subscribe_request = { command: "subscribe", identifier: { channel: channel }.to_json }

    # `client` represents a websocket client connected to a server
    client.send(subscribe_request)

    ack = {
      "identifier" => { channel: channel }.to_json, "type" => "confirm_subscription"
    }

    # `receive` method returns the first message from the incoming messages queue;
    # waits 5s when no messages available
    assert_equal ack, client.receive
  end

  # describe you bug scenario here
  scenario %{
    Should work
  } do
    # ...
  end
end

# Required setup/teardown
begin
  Anyt::RPC.start
  Anyt::Command.run if Anyt.config.command
  Anyt::Tests.run
ensure
  Anyt::Command.stop if Anyt.config.command
  Anyt::RPC.stop
end
