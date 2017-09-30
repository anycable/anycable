# frozen_string_literal: true

require 'spec_helper'

describe Anycable do
  it 'has a version number' do
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
end
