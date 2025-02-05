# frozen_string_literal: true

feature "Presence" do
  connect_handler("presence_subscribe") do |connection|
    connection.uid = connection.params["user_id"]
    true
  end

  channel(:implicit) do
    include AnyCable::Rails::Channel::Presence

    def subscribed
      stream_from "a"
      return unless connection.uid.present?

      # connection identifier is used as a presence ID,
      # the only stream added during subscription is used as a presence stream
      join_presence
    end

    private

    def user_presence_info
      {name: connection.params["name"]}
    end
  end

  channel(:explicit) do
    include AnyCable::Rails::Channel::Presence

    def subscribed
      stream_from "b"
      return unless connection.uid.present?

      join_presence(
        "b_p",
        id: connection.params["user_id"].reverse,
        info: {whois: connection.params["name"]}
      )
    end
  end

  let(:client2) do
    build_client(ignore: %w[ping welcome], qs: "test=presence_subscribe&user_id=2024&name=Bart", protocol: "actioncable-v1-ext-json")
  end

  let(:client2_clone) do
    build_client(ignore: %w[ping welcome], qs: "test=presence_subscribe&user_id=2024&name=Bart", protocol: "actioncable-v1-ext-json")
  end

  let(:client3) do
    build_client(ignore: %w[ping welcome], qs: "test=presence_subscribe&user_id=2025&name=Lisa", protocol: "actioncable-v1-ext-json")
  end

  scenario %(
    Join channel on subscribe with implicit identification and leave on disconnect
  ) do
    subscribe_request = {command: "subscribe", identifier: {channel: implicit_channel}.to_json}

    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: implicit_channel}.to_json, "type" => "confirm_subscription"
    }

    assert_message ack, client.receive

    client2.send(subscribe_request)
    assert_message ack, client2.receive

    join_message = {
      "identifier" => {channel: implicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "join",
        "id" => "2024",
        "info" => {"name" => "Bart"}
      }
    }

    assert_message join_message, client.receive
    assert_message join_message, client2.receive

    client3.send(subscribe_request)
    assert_message ack, client3.receive

    join_message = {
      "identifier" => {channel: implicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "join",
        "id" => "2025",
        "info" => {"name" => "Lisa"}
      }
    }

    assert_message join_message, client.receive
    assert_message join_message, client2.receive
    assert_message join_message, client3.receive

    client2.close

    leave_message = {
      "identifier" => {channel: implicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "leave",
        "id" => "2024"
      }
    }

    assert_message leave_message, client.receive
    assert_message leave_message, client3.receive
  end

  scenario %(
    Join channel with explicit presence data and track duplicate sessions
  ) do
    subscribe_request = {command: "subscribe", identifier: {channel: explicit_channel}.to_json}

    client3.send(subscribe_request)

    ack = {
      "identifier" => {channel: explicit_channel}.to_json, "type" => "confirm_subscription"
    }

    join_message = {
      "identifier" => {channel: explicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "join",
        "id" => "5202",
        "info" => {"whois" => "Lisa"}
      }
    }

    assert_message ack, client3.receive
    assert_message join_message, client3.receive

    client2.send(subscribe_request)
    assert_message ack, client2.receive

    join_message = {
      "identifier" => {channel: explicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "join",
        "id" => "4202",
        "info" => {"whois" => "Bart"}
      }
    }

    assert_message join_message, client3.receive
    assert_message join_message, client2.receive

    client2_clone.send(subscribe_request)
    assert_message ack, client2_clone.receive

    client2.close

    # no event should be sent
    assert_raises(Anyt::Client::TimeoutError) do
      msg = client3.receive(timeout: 0.5)
      raise "Client 1 should not receive any events, but got: #{msg}"
    end

    client2_clone.close

    leave_message = {
      "identifier" => {channel: explicit_channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "leave",
        "id" => "4202"
      }
    }

    assert_message leave_message, client3.receive
  end
end
