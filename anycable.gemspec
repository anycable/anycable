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
    "source_code_uri" => "http://github.com/anycable/anycable"
  }

  spec.executables = %w[anycable anycabled]
  spec.files = `git ls-files README.md MIT-LICENSE CHANGELOG.md lib bin`.split
  spec.require_paths = ["lib"]

  spec.required_ruby_version = ">= 2.5.0"

  spec.add_dependency "anyway_config", ">= 1.4.2"
  spec.add_dependency "grpc", "~> 1.17"

  spec.add_development_dependency "redis", ">= 4.0"
  spec.add_development_dependency "bundler", ">= 1"
  spec.add_development_dependency "rake", ">= 13.0"
  spec.add_development_dependency "rack", "~> 2.0"
  spec.add_development_dependency "rspec", ">= 3.5"
end
