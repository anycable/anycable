# frozen_string_literal: true

module AnyCable
  module Middlewares
    # Set sid for request.env from metadata
    class EnvSid < AnyCable::Middleware
      def call(_method, request, meta)
        return yield unless meta["sid"]
        request.env.sid = meta["sid"]

        yield
      end
    end
  end
end
