# frozen_string_literal: true

require "rack"
require "anycable"
require "anycable/cli"

require_relative "../support/test_factory"

AnyCable.connection_factory = AnyCable::TestFactory

cli = AnyCable::CLI.new(embedded: true)
cli.run(["--version-check-enabled=false"])

p "Server started"

at_exit { cli.shutdown }

loop {}
