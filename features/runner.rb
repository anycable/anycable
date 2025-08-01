# frozen_string_literal: true

retried = false
require "bundler/inline"

begin
  gemfile(retried, quiet: true) do
    source "https://rubygems.org"

    gem "logger"
    gem "ostruct"

    gem "childprocess", "~> 4.1"
    gem "jwt"
    gem "activesupport", "~> 7.0.0"
    gem "perfect_toml"
  end
rescue
  raise if retried
  retried = true
  retry
end

require "socket"
require "time"
require "json"
require "uri"
require "net/http"
require "fileutils"

require "active_support/message_verifier"

class BenchRunner
  LOG_LEVEL_TO_NUM = {
    error: 0,
    warn: 1,
    info: 2,
    debug: 3
  }.freeze

  # CI machines could be slow
  RUN_TIMEOUT = (ENV["CI"] || ENV["CODESPACES"]) ? 120 : 30

  def initialize
    @processes = {}
    @teardowns = []
    @pipes = {}
    @log_level = ENV["DEBUG"] == "true" ? LOG_LEVEL_TO_NUM[:debug] : LOG_LEVEL_TO_NUM[:info]
  end

  def load(script, path)
    instance_eval script, path, 0
  end

  def launch(name, cmd, env: {}, debug: ENV["DEBUG"] == "true", capture_output: false)
    log(:info) { "Launching background process: #{cmd}"}

    process = ChildProcess.build(*cmd.split(/\s+/))
    # set process environment variables
    process.environment.merge!(env)

    if capture_output
      r, w = IO.pipe
      process.io.stdout = w
      process.io.stderr = w
      pipes[name] = {r: r, w: w}
    else
      process.io.inherit! if debug
    end

    process.detach = true

    processes[name] = process
    process.start
  end

  def run(name, cmd, env: {}, clean_env: false, timeout: RUN_TIMEOUT)
    log(:info) { "Running command: #{cmd}" }

    r, w = IO.pipe

    cmd = cmd.is_a?(Array) ? cmd : cmd.split(/\s+/)

    process = ChildProcess.build(*cmd)
    # set process environment variables
    # Remove all ANYCABLE_ vars that could be set by the Makefile
    if clean_env
      ENV.each { |k, _| env[k] ||= nil if k.start_with?("ANYCABLE_") }
    end

    process.environment.merge!(env)
    process.io.stdout = w
    process.io.stderr = w

    processes[name] = process
    pipes[name] = {r: r, w: w}

    process.start

    w.close

    begin
      process.poll_for_exit(timeout)
    rescue ChildProcess::TimeoutError
      process.stop
      log(:debug) { "Output:\n#{stdout(name)}" }
      fail "Command expected to finish in #{timeout}s but is still running"
    end

    log(:info) { "Finished" }
    log(:debug) { "Output:\n#{stdout(name)}" }
  end

  def gops(pid)
    log(:info) { "Fetching Go process #{pid} stats... "}

    `gops stats #{pid}`.lines.each_with_object({}) do |line, acc|
      key, val = line.split(/:\s+/)
      acc[key] = val.to_i
    end
  end

  def wait_tcp(port, host: "127.0.0.1", timeout: 10)
    log(:info) { "Waiting for TCP server to start at #{port}" }

    listening = false
    while timeout > 0
      begin
        Socket.tcp(host, port, connect_timeout: 1).close
        listening = true
        log(:info) { "TCP server is listening at #{port}" }
        break
      rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, SocketError
      end

      Kernel.sleep 0.5
      timeout -= 0.5
    end

    fail "No server is listening at #{port}" unless listening
  end

  def pid(name)
    processes.fetch(name).pid
  end

  def stop(name)
    processes.fetch(name).stop
    pipes[name]&.fetch(:w)&.close
  end

  def stdout(name)
    pipes.fetch(name).then do |pipe|
      pipe[:data] ||= pipe[:r].read
    end
  end

  def sleep(time)
    log(:info) { "Wait for #{time}s" }
    Kernel.sleep time
  end

  def shutdown
    processes.each_value do |process|
      process.stop
    end

    teardowns.each(&:call)
    teardowns.clear
  end

  def retrying(delay: 1, attempts: 2, &block)
    begin
      block.call
    rescue => e
      attempts -= 1
      raise if attempts <= 0

      log(:info) { "Retrying after error: #{e.message}" }

      sleep delay
      retry
    end
  end

  def at_exit(&block)
    teardowns << block
  end

  def broadcast(stream, data, url: "http://localhost:8090/_broadcast", key: nil)
    uri = URI.parse(url)
    headers = {
      "Content-Type": "application/json"
    }
    if key
      headers["Authorization"] = "Bearer #{key}"
    end

    data = {stream: stream, data: data.to_json}
    http = Net::HTTP.new(uri.host, uri.port)
    request = Net::HTTP::Post.new(uri.request_uri, headers)
    request.body = data.to_json
    response = http.request(request)

    if response.code != "201"
      fail "Broadcast returned unexpected status: #{response.code}"
    end
  end

  private

  attr_reader :processes, :pipes, :log_level, :teardowns

  def log(level, &block)
    return unless log_level >= LOG_LEVEL_TO_NUM[level]

    $stdout.puts "[#{level}] [#{Time.now.iso8601}]  #{block.call}"
  end
end

if ARGF
  begin
    scripts = ARGF.each.group_by { ARGF.filename }
    scripts.each do |filename, lines|
      puts "\n--- RUN: #{filename} ---\n\n" if scripts.size > 1
      script = lines.join
      runner = BenchRunner.new

      begin
        runner.load(script, filename)
        puts "All OK 👍"
      rescue => e
        $stderr.puts e.message + "\n#{e.backtrace.take(5).join("\n") if ENV["DEBUG"] == "true"}"
        exit(1)
      ensure
        runner.shutdown
      end
    end
  rescue Errno::ENOENT
    puts "\n--- NOTHINIG TO EXECUTE ---\n\n"
  end
end
