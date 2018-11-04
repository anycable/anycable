# frozen_string_literal: true

require "fileutils"
require "spec_helper"

describe "CLI options", :cli do
  %w[INT TERM].each do |signal|
    it "terminates gracefully on SIG#{signal}" do
      run_cli("-r ../spec/dummies/app.rb") do |cli|
        expect(cli).to have_output_line("RPC server is listening")
        # wait before terminating
        sleep 0.5
        cli.signal(signal)
        expect(cli).to have_output_line("SIG#{signal} received")
        expect(cli).to have_output_line("Stopping...")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(0)
      end
    end
  end
end
