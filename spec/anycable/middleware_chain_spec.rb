# frozen_string_literal: true

require "spec_helper"

describe AnyCable::MiddlewareChain do
  let(:chain) { described_class.new }
  let(:middleware_class) { Class.new(AnyCable::Middleware) }

  it "handles classes" do
    chain.use(middleware_class)

    list = chain.to_a
    expect(list.size).to eq 1
    expect(list.first).to be_a(middleware_class)
  end

  it "handles instances" do
    instance = middleware_class.new

    chain.use(instance)

    expect(chain.to_a).to eq([instance])
  end

  it "raises when middleware has wrong class" do
    expect { chain.use(Class.new) }.to raise_error(
      ArgumentError,
      /must be a subclass of AnyCable::Middleware/
    )
  end

  describe "#freeze" do
    it "raises an error when trying to add middleware after freeze" do
      chain.freeze

      expect { chain.use(middleware_class) }.to raise_error(
        /Cannot modify AnyCable middlewares after server started/
      )
    end
  end
end
