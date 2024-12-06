source 'https://rubygems.org'
gemspec name: "anycable-core"

gem "debug", platform: :mri

gem "benchmark_driver"

# https://github.com/soutaro/steep/issues/466
gem "activesupport", "~> 6.0"

unless ENV["GRPC"] == "false"
  case ENV["ANYCABLE_GRPC_IMPL"]
  when "grpc_kit" then gem "grpc_kit"
  else gem "grpc", "~> 1.37"
  end
end

gem "async-websocket"

eval_gemfile "gemfiles/rubocop.gemfile"
eval_gemfile "gemfiles/rbs.gemfile"

local_gemfile = "#{File.dirname(__FILE__)}/Gemfile.local"

if File.exist?(local_gemfile)
  eval(File.read(local_gemfile)) # rubocop:disable Lint/Eval
end
