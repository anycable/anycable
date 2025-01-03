# Gemfile for running conformance tests with Anyt
source "https://rubygems.org"

gem "nats-pure", "< 2.3.0"
gem "colorize"
gem "puma"

gem "activesupport", "~> 7.0.0"

if File.directory?(File.join(__dir__, "../anycable-rb"))
  $stdout.puts "\n=== Using local gems for Anyt ===\n\n"
  gem "debug"
  gem "anycable", ">= 1.6.0-rc.1", path: "../anycable-rb"
  gem "anycable-rails", ">= 1.6.0-rc.1", path: "../anycable-rails"
  gem "anyt", path: "../anyt"
  gem "wsdirector-cli", path: "../wsdirector"
else
  gem "anycable", ">= 1.6.0-rc.1", github: "anycable/anycable-rb", branch: "main"
  gem "anycable-rails", ">= 1.6.0-rc.1", github: "anycable/anycable-rails", branch: "main"
  gem "anyt"
  gem "wsdirector-cli"
end
