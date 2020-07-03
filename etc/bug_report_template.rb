# frozen_string_literal: true

require "bundler/inline"

# This reproduction script is based on `anyt` gem
# (https://github.com/anycable/anyt).
#
# See more test examples here:
# https://github.com/anycable/anyt/tree/master/lib/anyt/tests

gemfile(true) do
  source "https://rubygems.org"

  # For AnyCable <1.0, use ~> 0.8
  gem "anyt", ">= 1.0.0"
end

# Use HTTP adapter by default
ENV["ANYCABLE_BROADCAST_ADAPTER"] = "http"

require "anyt"
require "anyt/cli"

# Test scenario
feature "issue_xyz" do
  # This block defines an anonymous channel to test against
  channel do
    def subscribed
      stream_from "a"
    end
    #
    # def perform(data)
    #  ...
    # end
  end

  # You can use minitest/spec features here
  before do
    # `channel` contains identifier of the anonymous channel defined above
    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}

    # `client` represents a websocket client connected to a server
    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    # `receive` method returns the first message from the incoming messages queue;
    # waits 5s when no messages available
    assert_equal ack, client.receive
  end

  # describe you bug scenario here
  scenario %(
    Should work
  ) do
    # ...
    ActionCable.server.broadcast "a", "test"

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => "test"
    }

    assert_equal msg, client.receive
  end
end

ARGV.clear

# WebSocket server url
ARGV << "--target-url=ws://localhost:8080/cable"
# Command to launch WebSocket server.
# Comment this line if you want to run WebSocket server manually
ARGV << "--command=anycable-go"
# Run only the scenarios specified in this file
ARGV << "--only=bug_report_template"

Anyt::Cli.run
