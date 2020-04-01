source 'https://rubygems.org'
gemspec

gem "pry-byebug", platform: :mri

gem "benchmark_driver"

gem "anyway_config", ENV.fetch("ANYWAY_CONFIG_VERSION", ">= 1.4.2")

eval_gemfile "gemfiles/rubocop.gemfile"

local_gemfile = "#{File.dirname(__FILE__)}/Gemfile.local"

if File.exist?(local_gemfile)
  eval(File.read(local_gemfile)) # rubocop:disable Lint/Eval
end
