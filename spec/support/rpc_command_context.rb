# frozen_string_literal: true

shared_context "rpc_command" do
  include_context "anycable:rpc:command"

  let(:user) { "john" }
  let(:url) { "ws://example.anycable.com/cable" }
  let(:identifiers) { {current_user: user, url: url} }
end
