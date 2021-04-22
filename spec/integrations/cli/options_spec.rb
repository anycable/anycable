# frozen_string_literal: true

require "spec_helper"

describe "CLI options", :cli do
  specify "-v" do
    run_cli("-v") do |cli|
      expect(cli).to have_output_line("v#{AnyCable::VERSION}")
      expect(cli).to have_stopped
      expect(cli).to have_exit_status(0)
    end
  end

  specify "--version" do
    run_cli("--version") do |cli|
      expect(cli).to have_output_line("v#{AnyCable::VERSION}")
      expect(cli).to have_stopped
      expect(cli).to have_exit_status(0)
    end
  end

  specify "--help" do
    run_cli("--help") do |cli|
      expect(cli).to have_output_line("$ anycable [options]")
      expect(cli).to have_output_line("CLI")
      expect(cli).to have_output_line("APPLICATION")
      expect(cli).to have_output_line("REDIS PUB/SUB")
      expect(cli).to have_output_line("HTTP PUB/SUB")
      expect(cli).to have_output_line("HTTP HEALTH CHECKER")
      expect(cli).to have_stopped
      expect(cli).to have_exit_status(0)
    end
  end

  specify "many options" do
    run_cli(
      "-r ../spec/dummies/app.rb " \
      "--redis-channel _test_cable_ --debug " \
      "--http-health-port 9009 --http-health-path '/hc'"
    ) do |cli|
      expect(cli).to have_output_line(
        'HTTP health server is listening on localhost:9009 and mounted at "/hc"'
      )
      expect(cli).to have_output_line("Broadcasting Redis channel: _test_cable_")
    end
  end

  specify "http broadcast options" do
    run_cli(
      "-r ../spec/dummies/app.rb " \
      "--broadcast-adapter=http " \
      "--http-broadcast-url=http://my-ws.com/_broadcast/me " \
      "--http-broadcast-secret=test"
    ) do |cli|
      expect(cli).to have_output_line("Broadcasting HTTP url: http://my-ws.com/_broadcast/me (with authorization)")
    end
  end
end
