# frozen_string_literal: true
require "spec_helper"
require "bg_helper"

describe "client connection" do
  include_context "rpc stub"

  subject { service.connect(request) }

  context "no cookies" do
    let(:request) { Anycable::ConnectionRequest.new }

    it "responds with error if no cookies" do
      expect(subject.status).to eq :ERROR
    end
  end

  context "with cookies and path info" do
    let(:request) do
      Anycable::ConnectionRequest.new(
        headers: {
          'Cookie' => 'username=john;'
        },
        path: 'http://example.io/cable?token=123'
      )
    end

    it "responds with success, correct identifiers and 'welcome' message", :aggregate_failures do
      expect(subject.status).to eq :SUCCESS
      identifiers = JSON.parse(subject.identifiers)
      expect(identifiers).to include(
        'current_user',
        'url' => 'http://example.io/cable?token=123'
      )
      expect(subject.transmissions.first).to eq JSON.dump('type' => 'welcome')
    end
  end
end
