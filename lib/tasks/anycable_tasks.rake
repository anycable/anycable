# frozen_string_literal: true
namespace :anycable do
  desc "Make test gRPC call"
  task check: :environment do
    require 'grpc'
    require 'anycable/rpc/rpc_services'

    Anycable.logger = Logger.new(STDOUT)
    stub = Anycable::RPC::Stub.new(Anycable.config.rpc_host, :this_channel_is_insecure)
    stub.connect(
      Anycable::ConnectionRequest.new(
        path: 'http://example.com',
        headers: { 'Cookie' => 'test=1;' }
      )
    )
  end
end
