# frozen_string_literal: true

feature "History" do
  channel do
    def subscribed
      stream_from "history_a"
    end
  end

  before do
    ActionCable.server.broadcast(
      "history_a",
      {data: {user_id: 1, status: "left"}}
    )

    ActionCable.server.broadcast(
      "history_a",
      {data: {user_id: 2, status: "join"}}
    )

    # Make sure broadcasts have been published
    sleep 2
  end

  scenario %(
    Fetching by timestamp
  ) do
    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}

    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_equal ack, client.receive

    history_request = {
      command: "history",
      identifier: {channel: channel}.to_json,
      history: {
        since: Time.now.to_i - 30
      }
    }

    client.send(history_request)

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 1, "status" => "left"}
      }
    }

    msg_2 = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 2, "status" => "join"}
      }
    }

    assert_message msg, client.receive
    assert_message msg_2, client.receive
  end

  scenario %(
    Fetching by offset
  ) do
    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}

    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_equal ack, client.receive

    ActionCable.server.broadcast(
      "history_a",
      {data: {user_id: 42, status: "alive"}}
    )

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 42, "status" => "alive"}
      }
    }

    received = client.receive

    assert_message msg, received

    assert_includes received, "stream_id"
    assert_includes received, "offset"
    assert_includes received, "epoch"

    another_client = build_client(ignore: %w[ping welcome])
    another_client.send(subscribe_request)
    assert_equal ack, another_client.receive

    history_request = {
      command: "history",
      identifier: {channel: channel}.to_json,
      history: {
        streams: {
          received["stream_id"] => {
            offset: received["offset"] - 1,
            epoch: received["epoch"]
          }
        }
      }
    }

    another_client.send(history_request)

    assert_message msg, another_client.receive
  end

  scenario %(
    Subscribing with history
  ) do
    subscribe_request = {
      command: "subscribe",
      identifier: {channel: channel}.to_json,
      history: {
        since: Time.now.to_i - 30
      }
    }

    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_equal ack, client.receive

    msg = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 1, "status" => "left"}
      }
    }

    msg_2 = {
      "identifier" => {channel: channel}.to_json,
      "message" => {
        "data" => {"user_id" => 2, "status" => "join"}
      }
    }

    assert_message msg, client.receive
    assert_message msg_2, client.receive
  end
end
