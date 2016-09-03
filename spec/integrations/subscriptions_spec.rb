# frozen_string_literal: true
require "spec_helper"
require "bg_helper"

describe "subscriptions", :rpc_command do
  include_context "rpc stub"

  let(:channel) { 'TestChannel' }

  describe "#subscribe" do
    let(:command) { 'subscribe' }
    let(:user) { User.new(name: 'john', secret: '123') }

    subject { service.subscribe(request) }

    context "reject subscription" do
      let(:user) { User.new(name: 'john', secret: '000') }

      it "responds with error and subscription rejection", :aggregate_failures do
        expect(subject.status).to eq :ERROR
        expect(subject.stream_from).to eq false
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include('reject_subscription')
      end
    end

    context "successful subscription" do
      it "responds with success and subscription confirmation", :aggregate_failures do
        expect(subject.status).to eq :SUCCESS
        expect(subject.stream_from).to eq true
        expect(subject.stream_id).to eq 'test'
        expect(subject.stop_streams).to eq false
        expect(subject.transmissions.first).to include('confirm_subscription')
      end
    end

    context "unknown channel" do
      let(:channel) { 'FakeChannel' }

      it "responds with error" do
        expect(subject.status).to eq :ERROR
      end
    end
  end

  describe "#unsubscribe" do
    let(:command) { 'unsubscribe' }

    subject { service.unsubscribe(request) }

    it "responds with stop_all_streams" do
      expect(subject.status).to eq :SUCCESS
      expect(subject.stop_streams).to eq true
    end
  end
end
