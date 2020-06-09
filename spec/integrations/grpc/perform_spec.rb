# frozen_string_literal: true

require "spec_helper"

class TestPerformChannel < AnyCable::TestFactory::Channel
  def follow(*)
    stream_from "user_#{connection.identifiers["current_user"]}"
    stream_from "all"
  end

  def unfollow(*)
    stop_stream_from "all"
  end

  def add(data)
    transmit result: (data["a"] + data["b"])
  end

  def add_with_cookie(data)
    transmit result: (data["a"] + request.cookies["c"].to_i)
  end

  def tick(*)
    session["count"] ||= 0
    session["count"] += 1
    transmit count: session["count"]
  end

  def itick(*)
    state["counter"] ||= ""
    state["counter"] += "a"
    transmit count: state["counter"]
  end

  private

  def session
    connection.request.session
  end
end

AnyCable::TestFactory.register_channel "test_perform", TestPerformChannel

describe "client messages" do
  include_context "anycable:rpc:server"
  include_context "rpc_command"

  let(:channel_id) { "test_perform" }

  describe "#perform" do
    let(:command) { "message" }
    let(:data) { {action: "add", a: 1, b: 2} }

    subject { service.command(request) }

    it "responds with result" do
      expect(subject).to be_success
      expect(subject.transmissions.size).to eq 1
      expect(subject.transmissions.first).to include({"result" => 3}.to_json)
    end

    context "with multiple stream_from" do
      let(:data) { {action: "follow"} }

      it "responds with streams", :aggregate_failures do
        expect(subject).to be_success
        expect(subject.streams).to contain_exactly("user_john", "all")
        expect(subject.stop_streams).to eq false
      end
    end

    context "when exception" do
      let(:data) { {action: "add", a: 1, b: "smth"} }

      it "responds with ERROR", :aggregate_failures do
        expect(subject).to be_error
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

    context "stop streams" do
      let(:data) { {action: "unfollow"} }

      it "responds with stopped streams", :aggregate_failures do
        expect(subject).to be_success
        expect(subject.stopped_streams).to contain_exactly("all")
        expect(subject.stop_streams).to eq false
      end
    end

    context "request headers access" do
      let(:headers) { {"cookie" => "c=3;"} }
      let(:data) { {action: "add_with_cookie", a: 5} }

      it "responds with result" do
        expect(subject).to be_success
        expect(subject.transmissions.size).to eq 1
        expect(subject.transmissions.first).to include({"result" => 8}.to_json)
      end
    end

    context "session persistence" do
      let(:data) { {action: "tick"} }

      it "persists session after each command" do
        first_call = service.command(request)

        expect(first_call).to be_success
        expect(first_call.transmissions.size).to eq 1
        expect(first_call.transmissions.first).to include({"count" => 1}.to_json)
        # the session has changed
        expect(first_call.session).not_to be_nil

        first_session = first_call.session

        request.session = first_session

        second_call = service.command(request)

        expect(second_call).to be_success
        expect(second_call.transmissions.size).to eq 1
        expect(second_call.transmissions.first).to include({"count" => 2}.to_json)
        # the session has changed
        expect(second_call.session).not_to be_nil

        expect(second_call.session).not_to eq first_session

        request.data = {action: "add", a: 1, b: 2}.to_json

        third_call = service.command(request)

        expect(third_call).to be_success
        # # performing a call that doesn't modify session shouldn't
        # # return anything
        expect(third_call.session).to be_nil
      end
    end

    context "channel state" do
      let(:data) { {action: "itick"} }

      it "persists session after each command" do
        first_call = service.command(request)

        expect(first_call).to be_success
        expect(first_call.transmissions.size).to eq 1
        expect(first_call.transmissions.first).to include({"count" => "a"}.to_json)
        # the istate has changed
        expect(first_call.istate.to_h).not_to be_empty

        first_istate = first_call.istate

        request.istate = first_istate

        second_call = service.command(request)

        expect(second_call).to be_success
        expect(second_call.transmissions.size).to eq 1
        expect(second_call.transmissions.first).to include({"count" => "aa"}.to_json)
        # the istate has changed
        expect(second_call.istate.to_h).not_to be_empty

        expect(second_call.istate).not_to eq first_istate

        request.data = {action: "add", a: 1, b: 2}.to_json

        third_call = service.command(request)

        expect(third_call).to be_success
        # erforming a call that doesn't modify instance vars
        # return nothing
        expect(third_call.istate.to_h).to be_empty
      end
    end
  end
end
