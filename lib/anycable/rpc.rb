# frozen_string_literal: true

require "anycable/rpc/rpc_pb"
require "anycable/rpc/rpc_services_pb"

# Extend some PB auto-generated classes
module AnyCable
  # Current RPC proto version (used for compatibility checks)
  PROTO_VERSION = "v1"
  SESSION_KEY = "_s_"

  # Add setters/getter for cstate field
  module WithConnectionState
    def initialize(session: nil, **other)
      if session
        other[:cstate] ||= {}
        other[:cstate][SESSION_KEY] = session
      end
      super(**other)
    end

    def session=(val)
      self.cstate = {} unless cstate
      cstate[SESSION_KEY] = val
    end

    def session
      cstate[SESSION_KEY]
    end

    def cstate
      env.cstate
    end

    def cstate=(val)
      env.cstate = val
    end
  end

  class ConnectionResponse
    prepend WithConnectionState
  end

  class CommandMessage
    prepend WithConnectionState
  end

  class CommandResponse
    prepend WithConnectionState
  end

  class DisconnectRequest
    prepend WithConnectionState
  end
end
