# frozen_string_literal: true

require "pty"

module CLITesting
  class CLIControl
    DEFAULT_WAIT_TIME = 5

    attr_reader :stdout, :stderr, :pid, :process_status

    def initialize(stdout, stderr, pid)
      @stdout = stdout
      @stderr = stderr
      @pid = pid
      @output = []
    end

    def signal(sig)
      Process.kill(sig, pid)
    end

    def has_stopped?(wait: DEFAULT_WAIT_TIME)
      loop do
        break true if stopped?
        break false if wait <= 0

        sleep 0.2
        wait -= 0.2
      end
    end

    def has_output_line?(text, wait: DEFAULT_WAIT_TIME)
      return true if output.any? { |line| line.include?(text) }

      line = nil

      loop do
        res = stdout.wait_readable(0.2)

        if res.nil?
          wait -= 0.2
        else
          begin
            line = stdout.gets&.chomp
          rescue Errno::EIO
            line = nil
          end

          raise_line_not_found!(text) if line.nil?
          output << line
          break true if line.include?(text)
        end

        raise_line_not_found!(text) if wait <= 0
      end
    end

    def has_exit_status?(status)
      raise "Process is still running" unless stopped?

      status == process_status.exitstatus
    end

    private

    attr_reader :output

    def raise_line_not_found!(line)
      raise RSpec::Expectations::ExpectationNotMetError, "Expected to have in the output line: #{line}, but had instead:" \
        "\n#{output.join("\n")}"
    end

    def stopped?
      return true unless process_status.nil?

      @process_status = PTY.check(pid)

      !process_status.nil?
    end
  end

  # Unset ANYCABLE_CONF to use defaults (to avoid RPC servers from the test process and
  # spawned process collision)
  def run_command(command, chdir: nil, env: {"ANYCABLE_CONF" => ""})
    rspex = nil
    ctrl = nil

    PTY.spawn(
      env,
      command,
      chdir: chdir || File.join(PROJECT_ROOT, "bin")
    ) do |stdout, stderr, pid|
      ctrl = CLIControl.new(stdout, stderr, pid)
      yield ctrl
    rescue Exception => e # rubocop:disable Lint/RescueException
      rspex = e
    ensure
      Process.kill("SIGKILL", pid)
      ctrl&.has_stopped?
    end
  rescue PTY::ChildExited, Errno::ESRCH
    # no-op
  ensure
    raise rspex unless rspex.nil?
  end

  def run_cli(opt_string = "", **opts, &block)
    run_command "bundle exec anycable #{opt_string}", **opts, &block
  end

  def run_ruby(opt_string = "", **opts, &block)
    run_command "bundle exec ruby #{opt_string}", **opts, &block
  end
end

RSpec.configure do |config|
  config.include CLITesting, cli: true
end
