# frozen_string_literal: true

# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: rpc.proto

require "google/protobuf"

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file("rpc.proto", syntax: :proto3) do
    add_message "anycable.Env" do
      optional :url, :string, 1
      map :headers, :string, :string, 2
      map :cstate, :string, :string, 3
      map :istate, :string, :string, 4
    end
    add_message "anycable.EnvResponse" do
      map :cstate, :string, :string, 1
      map :istate, :string, :string, 2
    end
    add_message "anycable.ConnectionRequest" do
      optional :env, :message, 3, "anycable.Env"
    end
    add_message "anycable.ConnectionResponse" do
      optional :status, :enum, 1, "anycable.Status"
      optional :identifiers, :string, 2
      repeated :transmissions, :string, 3
      optional :error_msg, :string, 4
      optional :env, :message, 5, "anycable.EnvResponse"
    end
    add_message "anycable.CommandMessage" do
      optional :command, :string, 1
      optional :identifier, :string, 2
      optional :connection_identifiers, :string, 3
      optional :data, :string, 4
      optional :env, :message, 5, "anycable.Env"
    end
    add_message "anycable.CommandResponse" do
      optional :status, :enum, 1, "anycable.Status"
      optional :disconnect, :bool, 2
      optional :stop_streams, :bool, 3
      repeated :streams, :string, 4
      repeated :transmissions, :string, 5
      optional :error_msg, :string, 6
      optional :env, :message, 7, "anycable.EnvResponse"
      repeated :stopped_streams, :string, 8
    end
    add_message "anycable.DisconnectRequest" do
      optional :identifiers, :string, 1
      repeated :subscriptions, :string, 2
      optional :env, :message, 5, "anycable.Env"
    end
    add_message "anycable.DisconnectResponse" do
      optional :status, :enum, 1, "anycable.Status"
      optional :error_msg, :string, 2
    end
    add_enum "anycable.Status" do
      value :ERROR, 0
      value :SUCCESS, 1
      value :FAILURE, 2
    end
  end
end

module AnyCable
  Env = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.Env").msgclass
  EnvResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.EnvResponse").msgclass
  ConnectionRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.ConnectionRequest").msgclass
  ConnectionResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.ConnectionResponse").msgclass
  CommandMessage = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.CommandMessage").msgclass
  CommandResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.CommandResponse").msgclass
  DisconnectRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.DisconnectRequest").msgclass
  DisconnectResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.DisconnectResponse").msgclass
  Status = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable.Status").enummodule
end
