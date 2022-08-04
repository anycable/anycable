# frozen_string_literal: true

begin
  require "nats/client"
rescue LoadError
  raise "Please, install the nats-pure gem to use NATS broadcast adapter"
end

require "json"

module AnyCable
  module BroadcastAdapters
    # NATS adapter for broadcasting.
    #
    # Example:
    #
    #   AnyCable.broadcast_adapter = :nats
    #
    # It uses NATS configuration from global AnyCable config
    # by default.
    #
    # You can override these params:
    #
    #   AnyCable.broadcast_adapter = :nats, servers: "nats://my_nats:4242", channel: "_any_cable_"
    class Nats < Base
      attr_reader :nats_conn, :channel

      def initialize(
        channel: AnyCable.config.nats_channel,
        **options
      )
        options = AnyCable.config.to_nats_params.merge(options)
        @nats_conn = ::NATS.connect(nil, options)
        @channel = channel
      end

      def raw_broadcast(payload)
        nats_conn.publish(channel, payload)
      end

      def announce!
        logger.info "Broadcasting NATS channel: #{channel}"
      end
    end
  end
end
