# frozen_string_literal: true

require_relative "lib/anycable/version"

Gem::Specification.new do |spec|
  spec.name = "anycable-core"
  spec.version = AnyCable::VERSION
  spec.authors = ["palkan"]
  spec.email = ["dementiev.vm@gmail.com"]

  spec.summary = "AnyCable core RPC implementation"
  spec.description = "AnyCable core RPC implementation not depenending on a particular server type (e.g., gRPC or whatever)"
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

  spec.executables = %w[anycable anycabled]
  spec.files = Dir.glob("lib/**/*") + Dir.glob("bin/*") + %w[README.md MIT-LICENSE CHANGELOG.md] +
    Dir.glob("sig/anycable/**/*.rbs") + %w[sig/anycable.rbs]
  spec.require_paths = ["lib"]

  spec.required_ruby_version = ">= 2.7.0"

  spec.add_dependency "anyway_config", "~> 2.2"
  spec.add_dependency "google-protobuf", ">= 3.13"

  spec.add_development_dependency "redis", ">= 4.0"
  spec.add_development_dependency "nats-pure", "~> 2"

  spec.add_development_dependency "bundler", ">= 1"
  spec.add_development_dependency "rake", ">= 13.0"
  spec.add_development_dependency "rack", "~> 2.0"
  spec.add_development_dependency "rspec", ">= 3.5"
  spec.add_development_dependency "simplecov"
  spec.add_development_dependency "simplecov-lcov"
  spec.add_development_dependency "webmock", "~> 3.8"
  spec.add_development_dependency "webrick"
end
