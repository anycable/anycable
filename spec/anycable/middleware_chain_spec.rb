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

  describe "#call" do
    before do
      # Add rescue middleware
      chain.use(Class.new(AnyCable::Middleware) do
        def call(mid, req)
          yield
        rescue => exp
          {status: :exception, message: exp.message}
        end
      end)

      # Add time tracking middleware
      chain.use(Class.new(AnyCable::Middleware) do
        def call(mid, req)
          start = Time.now
          yield.tap do |response|
            response[:duration] = Time.now - start
          end
        end
      end)

      # Add sleeping middleware
      chain.use(Class.new(AnyCable::Middleware) do
        def call(mid, req)
          if mid == :sleep
            sleep 1
          end
          yield
        end
      end)

      # Add aborting middleware
      chain.use(Class.new(AnyCable::Middleware) do
        def call(mid, req)
          if mid == :abort
            raise "Aborting from middleware"
          end
          yield
        end
      end)
    end

    specify "allows manipulating response" do
      response = chain.call(:test, "data") { {status: :ok} }

      expect(response[:status]).to eq :ok
      expect(response).to have_key(:duration)

      response = chain.call(:sleep, "data") { {status: :ok} }
      expect(response[:duration]).to be >= 1.0
    end

    specify "allows aborting calls via exceptions" do
      expect(chain.call(:abort, "data", &(proc {}))).to eq({status: :exception, message: "Aborting from middleware"})
      expect(chain.call(:test, "data") { raise "Handler exception" }).to eq({status: :exception, message: "Handler exception"})
    end
  end
end
