# frozen_string_literal: true

require "spec_helper"

describe "CLI options", :cli do
  describe "version" do
    specify "-v" do
      run_cli("-v") do |cli|
        expect(cli).to have_output_line("v#{Anycable::VERSION}")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(0)
      end
    end

    specify "--version" do
      run_cli("--version") do |cli|
        expect(cli).to have_output_line("v#{Anycable::VERSION}")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(0)
      end
    end

    specify "--help" do
      run_cli("--help") do |cli|
        expect(cli).to have_output_line("$ anycable [options]")
        expect(cli).to have_stopped
        expect(cli).to have_exit_status(0)
      end
    end
  end

  describe "server options" do
    specify "--rpc-host" do
      run_cli("--rpc-host localhost:50053 -r ../spec/support/dummy.rb") do |cli|
        expect(cli).to have_output_line("RPC server is listening on localhost:50053")
      end
    end
  end
end
