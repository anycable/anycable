# frozen_string_literal: true

shared_context "rpc command", rpc_command: true do
  let(:user) { "john" }
  let(:url) { "" }
  let(:command) { "" }
  let(:channel) { "" }
  let(:conn_id) { {current_user: user, url: url} }
  let(:data) { {} }
  let(:path) { "/cable" }
  let(:headers) { {} }
  let(:env) { {path: path, headers: headers} }
  let(:env_pb) { AnyCable::Env.new(**env) }

  let(:request) do
    AnyCable::CommandMessage.new(
      command: command,
      identifier: channel,
      connection_identifiers: conn_id.to_json,
      data: data.to_json,
      env: env_pb
    )
  end
end
