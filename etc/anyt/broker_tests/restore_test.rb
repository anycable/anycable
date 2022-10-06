# frozen_string_literal: true

feature "Session cache" do
  channel do
    def subscribed
      stream_from "cache_a"
    end
  end

  before do
    client = build_client(ignore: ["ping"])

    welcome_msg = client.receive
    assert_message({ "type" => "welcome" }, welcome_msg)

    assert_includes welcome_msg, "sid"

    @sid = welcome_msg["sid"]

    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}
    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_equal ack, client.receive

    ActionCable.server.broadcast(
      "cache_a",
      {data: {user_id: 1, status: "left"}}
    )

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 1, "status" => "left"}
      }
    }

    assert_message msg, client.receive
  end

  scenario %(
    Restore session by session ID
  ) do
    client.close

    another_client = build_client(
      ignore: ["ping"],
      headers: {
        "X-ANYCABLE-RESTORE-SID" => @sid
      }
    )

    assert_message({ "type" => "welcome", "restored" => true }, another_client.receive)

    ActionCable.server.broadcast(
      "cache_a",
      {data: {user_id: 2, status: "join"}}
    )

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 2, "status" => "join"}
      }
    }

    assert_message msg, another_client.receive
  end
end
