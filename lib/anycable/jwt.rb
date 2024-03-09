# frozen_string_literal: true

require "json"
require "openssl"

module AnyCable
  module JWT
    class DecodeError < StandardError; end

    class VerificationError < DecodeError; end

    class ExpiredSignature < DecodeError; end

    # Basic JWT encode/decode implementation suitable to our needs
    # and not requiring external dependencies
    module BasicImpl
      ALGORITHM = "HS256"

      class << self
        def encode(payload, secret_key)
          payload = ::Base64.urlsafe_encode64(payload.to_json, padding: false)
          headers = ::Base64.urlsafe_encode64({"alg" => ALGORITHM}.to_json, padding: false)

          header = "#{headers}.#{payload}"
          signature = sign(header, secret_key)

          "#{header}.#{signature}"
        end

        def decode(token, secret_key)
          header, payload, signature = token.split(".")
          # Check segments
          raise DecodeError, "Not enough or too many segments" unless header && payload && signature

          # Verify the algorithm
          decoded_header = ::JSON.parse(::Base64.urlsafe_decode64(header))
          raise DecodeError, "Algorithm not supported" unless decoded_header["alg"] == ALGORITHM

          # Verify the signature
          expected_signature = sign("#{header}.#{payload}", secret_key)
          raise VerificationError, "Signature verification failed" unless secure_compare(signature, expected_signature)

          # Verify expiration
          decoded_payload = ::JSON.parse(::Base64.urlsafe_decode64(payload))
          if decoded_payload.key?("exp")
            raise ExpiredSignature, "Signature has expired" if Time.now.to_i >= decoded_payload["exp"]
          end

          decoded_payload
        rescue JSON::ParserError, ArgumentError
          raise DecodeError, "Invalid segment encoding"
        end

        # We don't really care about timing attacks here,
        # since verification is done on the AnyCable server side.
        # But still, we can use constant-time comparison when it's available.
        if OpenSSL.respond_to?(:fixed_length_secure_compare)
          def secure_compare(a, b)
            return false if a.bytesize != b.bytesize

            OpenSSL.fixed_length_secure_compare(a, b)
          end
        else
          def secure_compare(a, b)
            return false if a.bytesize != b.bytesize

            a == b
          end
        end

        def sign(data, secret_key)
          ::Base64.urlsafe_encode64(
            ::OpenSSL::HMAC.digest("SHA256", secret_key, data),
            padding: false
          )
        end
      end
    end

    class << self
      attr_accessor :jwt_impl

      def encode(payload, expires_at: nil, secret_key: AnyCable.config.jwt_secret, ttl: AnyCable.config.jwt_ttl)
        raise ArgumentError, "JWT encryption key is not specified. Add it via `jwt_secret` or `secret` option" if secret_key.nil? || secret_key.empty?

        encoded = Serializer.serialize(payload).to_json

        data = {ext: encoded}

        data[:exp] = expires_at.to_i if expires_at

        if ttl&.positive? && !data.key?(:exp)
          data[:exp] = Time.now.to_i + ttl
        end

        jwt_impl.encode(data, secret_key)
      end

      def decode(token, secret_key: AnyCable.config.jwt_secret)
        raise ArgumentError, "JWT encryption key is not specified. Add it via `jwt_secret` or `secret` option" if secret_key.nil? || secret_key.empty?

        jwt_impl.decode(token, secret_key).then do |decoded|
          ::JSON.parse(decoded.fetch("ext"), symbolize_names: true)
        end.then { |data| Serializer.deserialize(data) }
      end
    end

    self.jwt_impl = BasicImpl
  end
end
