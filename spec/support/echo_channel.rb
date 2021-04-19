# frozen_string_literal: true

require_relative "./test_factory"

class EchoChannel < AnyCable::TestFactory::Channel
  def echo(data)
    transmit result: data
  end
end

AnyCable::TestFactory.register_channel "echo", EchoChannel
