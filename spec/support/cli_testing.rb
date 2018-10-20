# frozen_string_literal: true

require "pty"

module CLITesting
  class CLIControl
    DEFAULT_WAIT_TIME = 2

    attr_reader :stdout, :stderr, :pid

    def initialize(stdout, stderr, pid)
      @stdout = stdout
      @stderr = stderr
      @pid = pid
      @output = []
    end

    def has_stopped?(wait: DEFAULT_WAIT_TIME)
      loop do
        break true unless PTY.check(pid).nil?
        break false if wait <= 0

        sleep 0.2
        wait -= 0.2
      end
    end

    def has_output_line?(text, wait: DEFAULT_WAIT_TIME)
      return true if output.any? { |line| line.include?(text) }

      loop do
        line = stdout.gets&.chomp
        raise_line_not_found!(text) if line.nil?

        output << line
        break true if line.include?(text)

        raise_line_not_found!(text) if wait <= 0
        sleep 0.2
        wait -= 0.2
      end
    end

    def has_exit_status?(status)
      process_status = PTY.check(pid)
      raise "Process is still running" if process_status.nil?

      status == process_status.exitstatus
    end

    private

    attr_reader :output

    def raise_line_not_found!(line)
      raise RSpec::Expectations::ExpectationNotMetError, "Expected to have in the output line: #{line}, but had instead:" \
        "\n#{output.join("\n")}"
    end
  end

  def run_cli(opt_string = "", chdir: nil)
    PTY.spawn(
      "bundle exec anycable #{opt_string}",
      chdir: chdir || File.expand_path("../../bin", __dir__)
    ) do |stdout, stderr, pid|
      begin
        yield CLIControl.new(stdout, stderr, pid)
      ensure
        Process.kill("SIGKILL", pid) if PTY.check(pid).nil?
      end
    end
  rescue PTY::ChildExited, Errno::ESRCH
    # no-op
  end
end

RSpec.configure do |config|
  config.include CLITesting, cli: true
end
