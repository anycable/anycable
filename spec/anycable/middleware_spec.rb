# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Middleware do
  let(:middleware) do
    Class.new(described_class) do
      def call(_req, _call, method)
        return :error if method == "fail"

        yield
      end
    end.new
  end

  specify do
    expect(
      middleware.request_response(
        request: "test request",
        call: "test call",
        method: "success"
      ) { :ok }
    ).to eq(:ok)

    expect(
      middleware.request_response(
        request: "test request",
        call: "test call",
        method: "fail"
      )
    ).to eq :error
  end
end
