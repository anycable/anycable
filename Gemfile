# Gemfile for running conformance tests with Anyt
source "https://rubygems.org"

gem "nats-pure", "< 2.3.0"
gem "colorize"
gem "puma"

gem "activesupport", "~> 7.0.0"

if File.directory?(File.join(__dir__, "../anycable"))
  $stdout.puts "\n=== Using local gems for Anyt ===\n\n"
  gem "debug"
  gem "anycable", path: "../anycable"
  gem "anycable-rails", path: "../anycable-rails"
  gem "anyt", path: "../anyt"
  gem "wsdirector-cli", path: "../wsdirector"
else
  gem "anycable"
  gem "anycable-rails"
  gem "anyt"
  gem "wsdirector-cli"
end
