# frozen_string_literal: true
require "rails/generators/base"

class AnycableGenerator < Rails::Generators::Base # :nodoc:
  source_root File.expand_path('../templates', __FILE__)

  def create_executable_file
    template "script", "bin/anycable"
    chmod "bin/anycable", 0o755
  end
end
