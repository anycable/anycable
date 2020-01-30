# frozen_string_literal: true

RSpec.shared_context "anycable:rpc:command" do
  include_context "anycable:rpc:stub"

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
