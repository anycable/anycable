# frozen_string_literal: true

require "spec_helper"

describe AnyCable::RPC::Handlers::Command do
  include_context "rpc_command"

  let(:handler) { AnyCable::RPC::Handler.new }

  subject { handler.command(request) }

  describe "PERFORM" do
    before(:all) do
      Class.new(AnyCable::TestFactory::Channel) do
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
      end.tap { |klass| AnyCable::TestFactory.register_channel("test_perform", klass) }
    end

    after(:all) { AnyCable::TestFactory.unregister_channel("test_perform") }

    let(:channel_id) { "test_perform" }
    let(:command) { "message" }
    let(:data) { {action: "add", a: 1, b: 2} }

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

      it "raises an exception", :aggregate_failures do
        expect { subject }.to raise_error(/can't be coerced/)
      end
    end

    context "when stops streams" do
      let(:data) { {action: "unfollow"} }

      it "responds with stopped streams", :aggregate_failures do
        expect(subject).to be_success
        expect(subject.stopped_streams).to contain_exactly("all")
        expect(subject.stop_streams).to eq false
      end
    end

    context "with request headers" do
      let(:headers) { {"cookie" => "c=3;"} }
      let(:data) { {action: "add_with_cookie", a: 5} }

      it "responds with result" do
        expect(subject).to be_success
        expect(subject.transmissions.size).to eq 1
        expect(subject.transmissions.first).to include({"result" => 8}.to_json)
      end
    end

    context "with session persistence" do
      let(:data) { {action: "tick"} }

      it "persists session after each command" do
        first_call = handler.command(request)

        expect(first_call).to be_success
        expect(first_call.transmissions.size).to eq 1
        expect(first_call.transmissions.first).to include({"count" => 1}.to_json)
        # the session has changed
        expect(first_call.session).not_to be_nil

        first_session = first_call.session

        request.session = first_session

        second_call = handler.command(request)

        expect(second_call).to be_success
        expect(second_call.transmissions.size).to eq 1
        expect(second_call.transmissions.first).to include({"count" => 2}.to_json)
        # the session has changed
        expect(second_call.session).not_to be_nil

        expect(second_call.session).not_to eq first_session

        request.data = {action: "add", a: 1, b: 2}.to_json

        third_call = handler.command(request)

        expect(third_call).to be_success
        # # performing a call that doesn't modify session shouldn't
        # # return anything
        expect(third_call.session).to be_nil
      end
    end

    context "with channel state" do
      let(:data) { {action: "itick"} }

      it "persists session after each command" do
        first_call = handler.command(request)

        expect(first_call).to be_success
        expect(first_call.transmissions.size).to eq 1
        expect(first_call.transmissions.first).to include({"count" => "a"}.to_json)
        # the istate has changed
        expect(first_call.istate.to_h).not_to be_empty

        first_istate = first_call.istate

        request.istate = first_istate

        second_call = handler.command(request)

        expect(second_call).to be_success
        expect(second_call.transmissions.size).to eq 1
        expect(second_call.transmissions.first).to include({"count" => "aa"}.to_json)
        # the istate has changed
        expect(second_call.istate.to_h).not_to be_empty

        expect(second_call.istate.to_h).not_to eq first_istate.to_h

        request.data = {action: "add", a: 1, b: 2}.to_json

        third_call = handler.command(request)

        expect(third_call).to be_success
        # erforming a call that doesn't modify instance vars
        # return nothing
        expect(third_call.istate.to_h).to be_empty
      end
    end
  end

  describe "SUBSCRIBE" do
    before(:all) do
      Class.new(AnyCable::TestFactory::Channel) do
        def handle_subscribe
          super
          if connection.identifiers["current_user"] != "john"
            @rejected = true
          else
            stream_from "test"
          end
        end
      end.tap { |klass| AnyCable::TestFactory.register_channel("test_subscribe", klass) }
    end

    after(:all) { AnyCable::TestFactory.unregister_channel("test_subscribe") }

    let(:channel_id) { "test_subscribe" }

    let(:command) { "subscribe" }
    let(:user) { "john" }

    context "when subscription is rejected" do
      let(:user) { "jack" }

      it "responds with error and subscription rejection", :aggregate_failures do
        expect(subject).to be_failure
        expect(subject.streams).to eq []
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include("reject_subscription")
      end
    end

    context "when successful subscription" do
      it "responds with success and subscription confirmation", :aggregate_failures do
        expect(subject).to be_success
        expect(subject.streams).to eq ["test"]
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include("confirm_subscription")
      end
    end

    context "with unknown channel" do
      let(:channel_id) { "FakeChannel" }

      it "raises an exception" do
        expect { subject }.to raise_error(/unknown channel/i)
      end
    end
  end

  describe "UNSUBSCRIBE" do
    before(:all) do
      Class.new(AnyCable::TestFactory::Channel) do
        def handle_unsubscribe
          super
          AnyCable::TestFactory.log_event(
            identifier,
            user: connection.identifiers["current_user"],
            type: "unsubscribed"
          )
        end
      end.tap { |klass| AnyCable::TestFactory.register_channel("test_unsubscribe", klass) }
    end

    after(:all) { AnyCable::TestFactory.unregister_channel("test_unsubscribe") }

    let(:channel_id) { "test_unsubscribe" }

    let(:log) { AnyCable::TestFactory.events_log }

    let(:command) { "unsubscribe" }

    it "responds with stop_all_streams" do
      expect(subject).to be_success
      expect(subject.stop_streams).to eq true
      expect(subject.transmissions.first).to include("confirm_unsubscribe")
    end

    it "invokes #unsubscribed for channel" do
      expect { subject }
        .to change { log.select { |entry| entry[:source] == channel_id }.size }
        .by(1)

      channel_logs = log.select { |entry| entry[:source] == channel_id }
      expect(channel_logs.last[:data]).to eq(user: "john", type: "unsubscribed")
    end
  end

  context "with unknown command" do
    let(:channel_id) { "echo" }
    let(:command) { "fake" }

    it "raises an exception" do
      expect { subject }.to raise_error(/unknown command/i)
    end
  end
end
