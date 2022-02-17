# frozen_string_literal: true

require "logger"

class TestLogger < Logger
  def initialize
    reset
  end

  def reset
    @store = Hash.new { |h, k| h[k] = [] }
  end

  def [](severity)
    @store[severity]
  end

  Logger::Severity.constants.each do |severity|
    define_method severity.downcase do |msg|
      @store[severity.downcase] << msg
    end

    define_method("#{severity.downcase}?") { true }
  end
end
