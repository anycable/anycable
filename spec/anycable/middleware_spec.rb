# frozen_string_literal: true

require "spec_helper"

describe AnyCable::Middleware do
  let(:method) { :connect }

  let(:middleware) do
    Class.new(described_class) do
      def call(method, req)
        return :error if req == "fail"

        yield
      end
    end.new
  end

  it "yields" do
    expect(
      middleware.call(
        method,
        "test request"
      ) { :ok }
    ).to eq(:ok)

    expect(
      middleware.call(
        method,
        "fail"
      )
    ).to eq :error
  end
end
