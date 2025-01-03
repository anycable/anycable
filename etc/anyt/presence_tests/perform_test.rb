# frozen_string_literal: true

feature "Presence" do
  connect_handler("presence_perform") do |connection|
    connection.uid = connection.params["user_id"]
    true
  end

  channel do
    include AnyCable::Rails::Channel::Presence

    def subscribed
      stream_from "a"
    end

    def follow(data)
      join_presence "a", info: {name: data["name"]}
    end

    def unfollow
      leave_presence
    end
  end

  let(:client2) do
    build_client(ignore: %w[ping welcome], qs: "test=presence_perform&user_id=2024")
  end

  let(:client3) do
    build_client(ignore: %w[ping welcome], qs: "test=presence_perform&user_id=2025")
  end

  scenario %(
    Join and leave channel while performing actions
  ) do
    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}

    client.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_message ack, client.receive

    client2.send(subscribe_request)
    assert_message ack, client2.receive

    perform_request = {
      :command => "message",
      :identifier => {channel: channel}.to_json,
      "data" => {"action" => "follow", "name" => "Vova"}.to_json
    }

    client2.send(perform_request)

    join_message = {
      "identifier" => {channel: channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "join",
        "id" => "2024",
        "info" => {"name" => "Vova"}
      }
    }

    assert_message join_message, client.receive
    assert_message join_message, client2.receive

    perform_request = {
      :command => "message",
      :identifier => {channel: channel}.to_json,
      "data" => {"action" => "unfollow"}.to_json
    }

    client2.send(perform_request)

    leave_message = {
      "identifier" => {channel: channel}.to_json,
      "type" => "presence",
      "message" => {
        "type" => "leave",
        "id" => "2024"
      }
    }

    assert_message leave_message, client.receive
    assert_message leave_message, client2.receive
  end
end
