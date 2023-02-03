# frozen_string_literal: true

require "spec_helper"

describe "gRPC CLI options", :cli do
  specify "--help" do
    run_cli("--help") do |cli|
      expect(cli).to have_output_line("$ anycable [options]")
      expect(cli).to have_output_line("GRPC")
      expect(cli).to have_stopped
      expect(cli).to have_exit_status(0)
    end
  end

  describe "server options" do
    specify "--rpc-host" do
      run_cli("--rpc-host 0.0.0.0:50053 -r ../spec/dummies/app.rb") do |cli|
        expect(cli).to have_output_line("RPC server is listening on 0.0.0.0:50053")
      end
    end

    specify "many options" do
      run_cli(
        "--rpc-host 0.0.0.0:50053 -r ../spec/dummies/app.rb " \
        "--rpc-pool-size 10 --rpc-max-waiting-requests 2 " \
        "--rpc-poll-period 0.2 --rpc-pool-keep-alive 0.5 " \
        "--redis-channel _test_cable_ --debug "
      ) do |cli|
        expect(cli).to have_output_line("RPC server is listening on 0.0.0.0:50053")
        expect(cli).to have_output_line("Broadcasting Redis channel: _test_cable_")
        # GRPC logging
        if GRPC_KIT
          expect(cli).to have_output_line("Launched grpc_kit")
        else
          expect(cli).to have_output_line("handling /anycable.RPC/Connect with")
        end
      end
    end
  end
end
