# frozen_string_literal: true

require "spec_helper"

class TestPerformChannel < AnyCable::TestFactory::Channel
  def follow(*)
    stream_from "user_#{connection.identifiers['current_user']}"
    stream_from "all"
  end

  def add(data)
    transmit result: (data["a"] + data["b"])
  end
end

AnyCable::TestFactory.register_channel "test_perform", TestPerformChannel

describe "client messages", :with_grpc_server, :rpc_command do
  include_context "rpc stub"

  let(:channel) { "test_perform" }

  describe "#perform" do
    let(:command) { "message" }
    let(:data) { { action: "add", a: 1, b: 2 } }

    subject { service.command(request) }

    it "responds with result" do
      expect(subject.status).to eq :SUCCESS
      expect(subject.transmissions.size).to eq 1
      expect(subject.transmissions.first).to include({ "result" => 3 }.to_json)
    end

    context "with multiple stream_from" do
      let(:data) { { action: "follow" } }

      it "responds with streams", :aggregate_failures do
        expect(subject.status).to eq :SUCCESS
        expect(subject.streams).to contain_exactly("user_john", "all")
        expect(subject.stop_streams).to eq false
      end
    end

    context "when exception" do
      let(:data) { { action: "add", a: 1, b: "smth" } }

      it "responds with ERROR", :aggregate_failures do
        expect(subject.status).to eq :ERROR
        expect(subject.error_msg).to match(/can't be coerced/)
      end

      it "notifies exception handler" do
        subject

        expect(TestExHandler.last_error).to have_attributes(
          exception: have_attributes(message: a_string_matching(/can't be coerced/)),
          method: "command",
          message: request.to_h
        )
      end
    end
  end
end
