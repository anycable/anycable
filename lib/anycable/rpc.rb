# frozen_string_literal: true

require "anycable/rpc/rpc_pb"
require "anycable/rpc/rpc_services_pb"

require "anycable/rpc/helpers"
require "anycable/rpc/handlers/connect"
require "anycable/rpc/handlers/disconnect"
require "anycable/rpc/handlers/command"

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

    def istate
      env.istate
    end

    def istate=(val)
      env.istate = val
    end
  end

  # Status predicates
  module StatusPredicates
    def success?
      status == :SUCCESS
    end

    def failure?
      status == :FAILURE
    end

    def error?
      status == :ERROR
    end
  end

  class ConnectionResponse
    prepend WithConnectionState
    include StatusPredicates
  end

  class CommandMessage
    prepend WithConnectionState
  end

  class CommandResponse
    prepend WithConnectionState
    include StatusPredicates
  end

  class DisconnectRequest
    prepend WithConnectionState
  end

  class DisconnectResponse
    include StatusPredicates
  end
end
