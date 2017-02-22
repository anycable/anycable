# frozen_string_literal: true
Thread.new { Anycable::Server.start }

at_exit do
  Anycable::Server.stop
end
