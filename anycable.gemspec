# frozen_string_literal: true

lib = File.expand_path('../lib', __FILE__)
$LOAD_PATH.unshift(lib) unless $LOAD_PATH.include?(lib)
require 'anycable/version'

Gem::Specification.new do |spec|
  spec.name          = "anycable"
  spec.version       = Anycable::VERSION
  spec.authors       = ["palkan"]
  spec.email         = ["dementiev.vm@gmail.com"]

  spec.summary       = "Polyglot replacement for ActionCable server"
  spec.description   = "Polyglot replacement for ActionCable server"
  spec.homepage      = "http://github.com/anycable/anycable"
  spec.license       = "MIT"

  spec.files         = `git ls-files -z`.split("\x0").reject { |f| f.match(%r{^(test|spec|features)/}) }
  spec.require_paths = ["lib"]

  spec.add_dependency "anyway_config", "~> 1.0"
  spec.add_dependency "grpc", "~> 1.1"
  spec.add_dependency "redis", ">= 3.2"

  spec.add_development_dependency "bundler", "~> 1"
  spec.add_development_dependency "rake", ">= 10.0"
  spec.add_development_dependency "rack", "~> 2.0"
  spec.add_development_dependency "rspec", ">= 3.5"
  spec.add_development_dependency "rubocop", ">= 0.50"
  spec.add_development_dependency "simplecov", ">= 0.3.8"
  spec.add_development_dependency "pry-byebug"
end
