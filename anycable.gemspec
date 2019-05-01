# frozen_string_literal: true

lib = File.expand_path('../lib', __FILE__)
$LOAD_PATH.unshift(lib) unless $LOAD_PATH.include?(lib)
require 'anycable/version'

Gem::Specification.new do |spec|
  spec.name          = "anycable"
  spec.version       = AnyCable::VERSION
  spec.authors       = ["palkan"]
  spec.email         = ["dementiev.vm@gmail.com"]

  spec.summary       = "AnyCable is a polyglot replacement for ActionCable-compatible servers"
  spec.description   = "AnyCable is a polyglot replacement for ActionCable-compatible servers"
  spec.homepage      = "http://github.com/anycable/anycable"
  spec.license       = "MIT"

  spec.executables   = %w[anycable anycabled]
  spec.files         = `git ls-files README.md MIT-LICENSE CHANGELOG.md lib bin`.split
  spec.require_paths = ["lib"]

  spec.required_ruby_version = '>= 2.4.0'

  spec.add_dependency "anyway_config", "~> 1.4.2"
  spec.add_dependency "grpc", "~> 1.17"

  spec.add_development_dependency "redis", ">= 4.0"
  spec.add_development_dependency "bundler", ">= 1"
  spec.add_development_dependency "rake", ">= 10.0"
  spec.add_development_dependency "rack", "~> 2.0"
  spec.add_development_dependency "rspec", ">= 3.5"
  spec.add_development_dependency "rubocop", "~> 0.68.1"
  spec.add_development_dependency "simplecov", ">= 0.3.8"
  spec.add_development_dependency "pry-byebug"
end
