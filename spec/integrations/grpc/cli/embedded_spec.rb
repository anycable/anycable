# frozen_string_literal: true

require "spec_helper"

describe "CLI embedded", :cli do
  include_context "anycable:grpc:stub"

  let(:request) { AnyCable::ConnectionRequest.new(env: env) }

  subject { service.connect(request) }

  it "runs gRPC server" do
    # Pass env to avoid resetting config path (see support/cli_testing.rb)
    run_ruby("../spec/dummies/embedded.rb", env: {}) do |cli|
      expect(cli).to have_output_line("Server started")

      expect(subject).to be_failure
      expect(subject.transmissions).to eq(
        [JSON.dump({"type" => "disconnect", "reason" => "unauthorized"})]
      )
    end
  end
end
