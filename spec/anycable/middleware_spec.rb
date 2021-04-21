# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Middleware do
  let(:method) { AnyCable::GRPC::Handler.new.method(:connect) }

  let(:middleware) do
    Class.new(described_class) do
      def call(_req, call, method)
        return :error if call == "fail"

        yield
      end
    end.new
  end

  it "yields" do
    expect(
      middleware.request_response(
        request: "test request",
        call: "success",
        method: method
      ) { :ok }
    ).to eq(:ok)

    expect(
      middleware.request_response(
        request: "test request",
        call: "fail",
        method: method
      )
    ).to eq :error
  end

  context "when method is not from AnyCable service" do
    let(:method) { "".method(:to_s) }

    it "isn't called" do
      expect(
        middleware.request_response(
          request: "test request",
          call: "fail",
          method: method
        ) { :ok }
      ).to eq :ok
    end
  end
end
