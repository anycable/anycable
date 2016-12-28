# frozen_string_literal: true
module Anycable
  module Refinements
    module Subscriptions # :nodoc:
      refine ActionCable::Connection::Subscriptions do
        # Find or add a subscription to the list
        def fetch(identifier)
          add("identifier" => identifier) unless subscriptions[identifier]
          subscriptions[identifier]
        end
      end
    end
  end
end
