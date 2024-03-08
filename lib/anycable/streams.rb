# frozen_string_literal: true

require "openssl"
require "base64"
require "json"

module AnyCable
  module Streams
    class << self
      def signed(stream_name)
        ::Base64.urlsafe_encode64(::JSON.dump(stream_name)).then do |encoded|
          "#{encoded}--#{signature(encoded)}"
        end
      end

      def verified(signed_stream_name)
        encoded, sig = signed_stream_name.split("--")

        return unless sig == signature(encoded)

        ::Base64.urlsafe_decode64(encoded).then do |decoded|
          ::JSON.parse(decoded)
        end
      end

      private

      def signature(val)
        key = AnyCable.config.streams_secret

        raise ArgumentError, "streams signing secret is missing" unless key

        ::OpenSSL::HMAC.hexdigest("SHA256", key, val)
      end
    end
  end
end
