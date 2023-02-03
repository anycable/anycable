# frozen_string_literal: true

require "bundler/inline"

retried = false
begin
  gemfile(retried, quiet: true) do
    source "https://rubygems.org"

    gem "anycable", path: "../.."

    gem "actioncable"

    if File.directory?(File.join(__dir__, "../../../anycable-rails"))
      $stdout.puts "\n=== Using local anycable-rails gem ==="
      gem "anycable-rails", path: "../../../anycable-rails", require: false
    else
      gem "anycable-rails", require: false
    end

    if File.directory?(File.join(__dir__, "../../../anyt"))
      $stdout.puts "\n=== Using local anyt gem ==="
      gem "anyt", path: "../../../anyt"
    else
      gem "anyt"
    end
  end
rescue Gem::MissingSpecError
  raise if retried

  retried = true
  retry
end

require "anyt"

AnyCable.config.log_level = :error unless AnyCable.config.debug?

require "anyt/cli"

Anyt::Cli.run(["--only-rpc"])
