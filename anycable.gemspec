# frozen_string_literal: true

require_relative "lib/anycable/version"

Gem::Specification.new do |spec|
  spec.name = "anycable"
  spec.version = AnyCable::VERSION
  spec.authors = ["palkan"]
  spec.email = ["dementiev.vm@gmail.com"]

  spec.summary = "AnyCable is a polyglot replacement for ActionCable-compatible servers"
  spec.description = "AnyCable is a polyglot replacement for ActionCable-compatible servers"
  spec.homepage = "http://github.com/anycable/anycable"
  spec.license = "MIT"
  spec.metadata = {
    "bug_tracker_uri" => "http://github.com/anycable/anycable/issues",
    "changelog_uri" => "https://github.com/anycable/anycable/blob/master/CHANGELOG.md",
    "documentation_uri" => "https://docs.anycable.io/",
    "homepage_uri" => "https://anycable.io/",
    "source_code_uri" => "http://github.com/anycable/anycable",
    "funding_uri" => "https://github.com/sponsors/anycable"
  }

  spec.executables = []
  spec.files = %w[README.md MIT-LICENSE CHANGELOG.md]
  spec.require_paths = ["lib"]

  spec.required_ruby_version = ">= 2.7.0"

  spec.add_dependency "anycable-core", AnyCable::VERSION
  spec.add_dependency "grpc", "~> 1.37"
  # This is a minimal version that supports integer enums in JSON
  # required by HTTRPC: https://github.com/protocolbuffers/protobuf/pull/11673
  spec.add_dependency "google-protobuf", "~> 3.23"
end
