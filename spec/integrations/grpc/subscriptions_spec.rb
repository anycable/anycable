# frozen_string_literal: true

require "spec_helper"

describe "subscriptions" do
  include_context "anycable:grpc:server"
  include_context "anycable:grpc:stub"

  let(:channel_id) { "echo" }

  describe "#subscribe" do
    let(:command) { "subscribe" }
    let(:user) { "john" }

    subject { service.command(request) }

    it "responds with success and subscription confirmation", :aggregate_failures do
      expect(subject).to be_success
      expect(subject.streams).to eq []
      expect(subject.stop_streams).to eq false
      expect(subject.transmissions.first).to include("confirm_subscription")
    end

    it "invokes Command handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:command, request, an_instance_of(Hash))
    end
  end

  describe "#unsubscribe" do
    let(:log) { AnyCable::TestFactory.events_log }

    let(:command) { "unsubscribe" }

    subject { service.command(request) }

    it "responds with stop_all_streams" do
      expect(subject).to be_success
      expect(subject.stop_streams).to eq true
      expect(subject.transmissions.first).to include("confirm_unsubscribe")
    end

    it "invokes Command handler" do
      allow(AnyCable.rpc_handler).to receive(:handle).and_call_original

      expect(subject).to be_success
      expect(AnyCable.rpc_handler).to have_received(:handle).with(:command, request, an_instance_of(Hash))
    end
  end

  context "exception handling" do
    let(:channel_id) { "fecho" }

    subject { service.command(request) }

    it "responds with error" do
      expect(subject).to be_error
    end

    it "notifies exception handler" do
      subject

      expect(TestExHandler.last_error).to have_attributes(
        exception: have_attributes(message: "Unknown channel: fecho"),
        method: "command",
        message: request.to_h
      )
    end
  end
end
