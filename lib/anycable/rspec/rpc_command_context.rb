# frozen_string_literal: true

RSpec.shared_context "anycable:rpc:command" do
  let(:url) { "ws://example.anycable.com/cable" }
  let(:headers) { {} }
  let(:env) { AnyCable::Env.new(url: url, headers: headers) }
  let(:command) { "" }
  let(:channel_id) { "" }
  let(:identifiers) { {} }
  let(:data) { {} }

  let(:request) do
    AnyCable::CommandMessage.new(
      command: command,
      identifier: channel_id,
      connection_identifiers: identifiers.to_json,
      data: data.to_json,
      env: env
    )
  end
end
