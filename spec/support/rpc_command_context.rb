# frozen_string_literal: true
shared_context "rpc command", rpc_command: true do
  let(:user) { User.new(name: 'john', secret: '123') }
  let(:url) { '' }
  let(:command) { '' }
  let(:channel) { '' }
  let(:conn_id) { { current_user: user.to_gid_param, url: url } }
  let(:data) { {} }

  let(:channel_params) { {} }
  let(:channel_id) { { channel: channel }.merge(channel_params) }
  let(:channel_id_json) { channel_id.to_json }

  let(:request) do
    Anycable::CommandMessage.new(
      command: command,
      identifier: channel_id_json,
      connection_identifiers: conn_id.to_json,
      data: data.to_json
    )
  end
end
