# frozen_string_literal: true

require "anycable/protos/rpc_pb"

require "anycable/rpc/handler"

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
      state_ = cstate
      if state_
        state_[SESSION_KEY] = val
      end
    end

    def session
      state_ = cstate
      if state_
        state_[SESSION_KEY]
      end
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
