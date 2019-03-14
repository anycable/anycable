# frozen_string_literal: true

require "spec_helper"

class TestSubscriptionsChannel < AnyCable::TestFactory::Channel
  def handle_subscribe
    if connection.identifiers["current_user"] != "john"
      @rejected = true
      connection.transmit(type: "reject_subscription", identifier: identifier)
    else
      stream_from "test"
      connection.transmit(type: "confirm_subscription", identifier: identifier)
    end
  end

  def handle_unsubscribe
    stop_all_streams
    AnyCable::TestFactory.log_event(
      identifier,
      user: connection.identifiers["current_user"],
      type: "unsubscribed"
    )
    transmit(type: "confirm_unsubscribe", identifier: identifier)
  end
end

AnyCable::TestFactory.register_channel "test_subscriptions", TestSubscriptionsChannel

describe "subscriptions", :with_grpc_server, :rpc_command do
  include_context "rpc stub"

  let(:channel) { "test_subscriptions" }

  describe "#subscribe" do
    let(:command) { "subscribe" }
    let(:user) { "john" }

    subject { service.command(request) }

    context "reject subscription" do
      let(:user) { "jack" }

      it "responds with error and subscription rejection", :aggregate_failures do
        expect(subject.status).to eq :FAILURE
        expect(subject.streams).to eq []
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include("reject_subscription")
      end
    end

    context "successful subscription" do
      it "responds with success and subscription confirmation", :aggregate_failures do
        expect(subject.status).to eq :SUCCESS
        expect(subject.streams).to eq ["test"]
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include("confirm_subscription")
      end
    end

    context "unknown channel" do
      let(:channel) { "FakeChannel" }

      it "responds with error" do
        expect(subject.status).to eq :ERROR
      end
    end
  end

  describe "#unsubscribe" do
    let(:log) { AnyCable::TestFactory.events_log }

    let(:command) { "unsubscribe" }

    subject { service.command(request) }

    it "responds with stop_all_streams" do
      expect(subject.status).to eq :SUCCESS
      expect(subject.stop_streams).to eq true
      expect(subject.transmissions.first).to include("confirm_unsubscribe")
    end

    it "invokes #unsubscribed for channel" do
      expect { subject }
        .to change { log.select { |entry| entry[:source] == channel }.size }
        .by(1)

      channel_logs = log.select { |entry| entry[:source] == channel }
      expect(channel_logs.last[:data]).to eq(user: "john", type: "unsubscribed")
    end
  end

  context "exception handling" do
    let(:command) { "fake" }

    subject { service.command(request) }

    it "responds with error" do
      expect(subject.status).to eq :ERROR
    end

    it "notifies exception handler" do
      subject

      expect(TestExHandler.last_error).to have_attributes(
        exception: have_attributes(message: "Unknown command"),
        method: "command",
        message: request.to_h
      )
    end
  end
end
