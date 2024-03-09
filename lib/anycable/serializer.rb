# frozen_string_literal: true

module AnyCable
  # Serializer is responsible for converting Ruby objects to and from a transferrable format (e.g., for identifiers, connection/channel state, etc.).
  # It relies on configurable `.object_serializer` to handle non-primitive values and handles Hash/Array seririlzation.
  module Serializer
    class << self
      attr_accessor :object_serializer

      def serialize(obj)
        handled = object_serializer&.serialize(obj)
        return handled if handled

        case obj
        when nil, true, false, Integer, Float, String, Symbol
          obj
        when Hash
          obj.transform_values { |v| serialize(v) }
        when Array
          obj.map { |v| serialize(v) }
        else
          raise ArgumentError, "Can't serialize #{obj.inspect}"
        end
      end

      # Deserialize previously serialized value to a Ruby object.
      def deserialize(val)
        if val.is_a?(::String)
          handled = object_serializer&.deserialize(val)
          return handled if handled
        end

        case val
        when Hash
          val.transform_values { |v| deserialize(v) }
        when Array
          val.map { |v| deserialize(v) }
        else
          val
        end
      end
    end
  end
end
