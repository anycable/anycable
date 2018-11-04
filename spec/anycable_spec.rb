# frozen_string_literal: true

require "spec_helper"

describe Anycable do
  it "has a version number" do
    expect(described_class::VERSION).not_to be nil
  end

  describe "#logger" do
    around do |ex|
      old_logger = Anycable.logger
      Anycable.remove_instance_variable(:@logger)
      ex.run
      Anycable.logger = old_logger
      Anycable.config.reload
    end

    it "writes to stdout" do
      expect { Anycable.logger.info("Wow! Error!") }
        .to output(/Wow! Error!/).to_stdout_from_any_process
    end

    context "logging to file" do
      before { Anycable.config.log_file = "tmp/test_anycable.log" }
      after { FileUtils.rm("tmp/test_anycable.log") }

      it "writes log to file" do
        Anycable.logger.info "Wow! Error!"

        expect(File.read("tmp/test_anycable.log")).to include("Wow! Error!")
      end
    end
  end

  describe "#broadcast_adapter=" do
    before(:all) do
      class Anycable::BroadcastAdapters::MyCustomAdapter
        attr_reader :options
        def initialize(options)
          @options = options
        end

        def broadcast; end
      end
    end

    after(:all) { Anycable::BroadcastAdapters.send(:remove_const, :MyCustomAdapter) }

    around do |ex|
      old_adapter = Anycable.broadcast_adapter
      Anycable.remove_instance_variable(:@broadcast_adapter)
      ex.run
      Anycable.broadcast_adapter = old_adapter
    end

    specify "redis by default" do
      expect(Anycable.instance_variable_defined?(:@broadcast_adapter)).to eq false
      expect(Anycable.broadcast_adapter).to be_a(Anycable::BroadcastAdapters::Redis)
    end

    specify "set by symbol" do
      Anycable.broadcast_adapter = :my_custom_adapter
      expect(Anycable.broadcast_adapter).to be_a(Anycable::BroadcastAdapters::MyCustomAdapter)
      expect(Anycable.broadcast_adapter.options).to eq({})
    end

    specify "set by symbol with options" do
      Anycable.broadcast_adapter = :my_custom_adapter, { url: "example.com" }
      expect(Anycable.broadcast_adapter).to be_a(Anycable::BroadcastAdapters::MyCustomAdapter)
      expect(Anycable.broadcast_adapter.options).to eq(url: "example.com")
    end

    specify "set by instance" do
      adapter = double("adapter", broadcast: nil)
      Anycable.broadcast_adapter = adapter
      Anycable.broadcast "test", "abc"
      expect(adapter).to have_received(:broadcast).with("test", "abc")
    end

    specify "raises error when adapter doesn't implement #broadcast method" do
      adapter = double("adapter")
      expect { Anycable.broadcast_adapter = adapter }
        .to raise_error(ArgumentError, /must implement #broadcast/)
    end

    specify "raises when adapter not found" do
      expect { Anycable.broadcast_adapter = :not_found }
        .to raise_error(LoadError, /Couldn't load the 'not_found' broadcast adapter for AnyCable/)
    end
  end
end
