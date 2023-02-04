# frozen_string_literal: true

# This file contains code to monitor a given process container memory usage
# and print the chart at exit

# Example usage:
#
#   ruby benchmarks/monitor_docker.rb $(docker ps | grep simple-cable-app_rails | awk '{ print $1 }')

DOCKER_ID = ENV["MONITOR_DOCKER_ID"] || ARGV[0]

return unless DOCKER_ID

REPORT_NAME = ENV["REPORT"]

def log(msg)
  puts "[#{Time.now}]\t#{msg}"
end

# Only use bundler/inline if gem is not installed yet
begin
  require "unicode_plot"
rescue LoadError
  require "bundler/inline"

  gemfile(true, quiet: true) do
    source "https://rubygems.org"

    gem "unicode_plot"
  end

  require "unicode_plot"
end

module DockerMonitor
  module_function

  def monitor_docker(id)
    report = {stopped: false, data: []}
    Thread.new do
      IO.popen("docker stats #{id} --format='{{.MemUsage}}' 2> /dev/null") do |io|
        while line = io.gets
          break if report[:stopped]
          report[:data] << line.match(/([\.\d]+)MiB/)[1].to_f
        end
      end
    end
    report
  end

  def print_report(data)
    stats = []

    # Print 10 values
    data.each_slice((data.size / 10) + 1) do |slice|
      stats << slice.max
    end

    log "\n📊 Memory snapshots: #{stats.join("\t")}\n"
  end

  def render_chart(data)
    stats = []

    # Print 10 values
    data.each_slice((data.size / 1_000) + 1) do |slice|
      stats << slice.max
    end

    plot = UnicodePlot.lineplot(stats, name: "MiB", width: [stats.size, 100].min, height: 20, color: :red)
    plot.render
  end

  def dump(data)
    require 'fileutils'

    Dir.chdir(File.join(__dir__, "..")) do
      FileUtils.mkdir("tmp") unless File.directory?("tmp")

      filename = ["tmp/#{Time.now.to_s.gsub(/\D/, '')}", REPORT_NAME, "mem.txt"].compact.join("_")

      log "Raw data is dumped to #{filename}"

      File.write(
        filename,
        data.map(&:to_s).join("\n")
      )
    end
  end
end

log "📈  Monitoring Docker container: #{DOCKER_ID}"

mem_report = DockerMonitor.monitor_docker(DOCKER_ID)

puts "Press any key to stop"
$stdin.gets

mem_report[:stopped] = true

DockerMonitor.print_report(mem_report[:data])

DockerMonitor.dump(mem_report[:data])

DockerMonitor.render_chart(mem_report[:data])
