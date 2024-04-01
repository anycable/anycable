# frozen_string_literal: true

feature "Whisper" do
  channel do
    def subscribed
      stream_from "a", whisper: true
    end
  end

  let(:client2) { build_client(ignore: %w[ping welcome]) }
  let(:client3) { build_client(ignore: %w[ping welcome]) }

  before do
    subscribe_request = {command: "subscribe", identifier: {channel: channel}.to_json}

    client.send(subscribe_request)
    client2.send(subscribe_request)
    client3.send(subscribe_request)

    ack = {
      "identifier" => {channel: channel}.to_json, "type" => "confirm_subscription"
    }

    assert_message ack, client.receive
    assert_message ack, client2.receive
    assert_message ack, client3.receive
  end

  scenario %(
    Only other clients receive the whisper message
  ) do
    perform_request = {
      :command => "whisper",
      :identifier => {channel: channel}.to_json,
      "data" => {"event" => "typing", "user" => "Vova"}
    }

    client.send(perform_request)

    msg = {"identifier" => {channel: channel}.to_json, "message" => {"event" => "typing", "user" => "Vova"}}

    assert_message msg, client2.receive
    assert_message msg, client3.receive
    assert_raises(Anyt::Client::TimeoutError) do
      msg = client.receive(timeout: 0.5)
      raise "Client 1 should not receive the message: #{msg}"
    end
  end
end
