source 'https://rubygems.org'
gemspec name: "anycable-core"

gem "pry-byebug", platform: :mri

gem "benchmark_driver"

# https://github.com/soutaro/steep/issues/466
gem "activesupport", "~> 6.0"

gem "anyway_config", ENV.fetch("ANYWAY_CONFIG_VERSION", ">= 2.1.0")
gem "grpc", "~> 1.37" unless ENV["GRPC"] == "false"

eval_gemfile "gemfiles/rubocop.gemfile"

local_gemfile = "#{File.dirname(__FILE__)}/Gemfile.local"

if File.exist?(local_gemfile)
  eval(File.read(local_gemfile)) # rubocop:disable Lint/Eval
end
