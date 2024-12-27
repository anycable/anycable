# frozen_string_literal: true

require "spec_helper"

describe "client messages" do
  include_context "anycable:grpc:server"
  include_context "anycable:grpc:stub"

  let(:channel_id) { "echo" }
  let(:command) { "message" }
  let(:data) { {action: "echo", data: 3} }

  subject { service.command(request) }

  it "responds with result" do
    expect(subject).to be_success
    expect(subject.transmissions.size).to eq 1
    expect(subject.transmissions.first).to include({"result" => {"data" => 3}}.to_json)
  end

  it "invokes Command handler" do
    allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

    expect(subject).to be_success
    expect(AnyCable.rpc_handler).to have_received(:handle).with(:command, request, an_instance_of(Hash))
  end

  context "when exception" do
    let(:data) { {action: "fecho"} }

    it "responds with ERROR", :aggregate_failures do
      expect(subject).to be_error
      expect(subject.error_msg).to match(/undefined method [`']fecho'/)
    end

    it "notifies exception handler" do
      subject

      expect(TestExHandler.last_error).to have_attributes(
        exception: have_attributes(message: a_string_matching(/undefined method [`']fecho'/)),
        method: "command",
        message: request.to_h
      )
    end
  end
end
