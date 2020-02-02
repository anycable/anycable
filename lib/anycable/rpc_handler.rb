# frozen_string_literal: true

require "anycable/socket"
require "anycable/rpc"

# rubocop:disable Metrics/AbcSize
# rubocop:disable Metrics/MethodLength
# rubocop:disable Metrics/ClassLength
module AnyCable
  # RPC service handler
  class RPCHandler < AnyCable::RPC::Service
    # Handle connection request from WebSocket server
    def connect(request, _unused_call)
      logger.debug("RPC Connect: #{request.inspect}")

      socket = build_socket(env: rack_env(request.env))

      connection = factory.call(socket)

      connection.handle_open

      if socket.closed?
        AnyCable::ConnectionResponse.new(status: AnyCable::Status::FAILURE)
      else
        AnyCable::ConnectionResponse.new(
          status: AnyCable::Status::SUCCESS,
          identifiers: connection.identifiers_json,
          transmissions: socket.transmissions,
          env: build_env_response(socket)
        )
      end
    rescue => exp
      notify_exception(exp, :connect, request)

      AnyCable::ConnectionResponse.new(
        status: AnyCable::Status::ERROR,
        error_msg: exp.message
      )
    end

    def disconnect(request, _unused_call)
      logger.debug("RPC Disconnect: #{request.inspect}")

      socket = build_socket(env: rack_env(request.env))

      connection = factory.call(
        socket,
        identifiers: request.identifiers,
        subscriptions: request.subscriptions
      )

      if connection.handle_close
        AnyCable::DisconnectResponse.new(status: AnyCable::Status::SUCCESS)
      else
        AnyCable::DisconnectResponse.new(status: AnyCable::Status::FAILURE)
      end
    rescue => exp
      notify_exception(exp, :disconnect, request)

      AnyCable::DisconnectResponse.new(
        status: AnyCable::Status::ERROR,
        error_msg: exp.message
      )
    end

    def command(message, _unused_call)
      logger.debug("RPC Command: #{message.inspect}")

      socket = build_socket(env: rack_env(message.env))

      connection = factory.call(
        socket,
        identifiers: message.connection_identifiers
      )

      result = connection.handle_channel_command(
        message.identifier,
        message.command,
        message.data
      )

      AnyCable::CommandResponse.new(
        status: result ? AnyCable::Status::SUCCESS : AnyCable::Status::FAILURE,
        disconnect: socket.closed?,
        stop_streams: socket.stop_streams?,
        streams: socket.streams,
        transmissions: socket.transmissions,
        env: build_env_response(socket)
      )
    rescue => exp
      notify_exception(exp, :command, message)

      AnyCable::CommandResponse.new(
        status: AnyCable::Status::ERROR,
        error_msg: exp.message
      )
    end

    private

    # Build Rack env from request
    def rack_env(request_env)
      uri = URI.parse(request_env.url)

      env = base_rack_env
      env.merge!(
        "PATH_INFO" => uri.path,
        "QUERY_STRING" => uri.query,
        "SERVER_NAME" => uri.host,
        "SERVER_PORT" => uri.port.to_s,
        "HTTP_HOST" => uri.host,
        "REMOTE_ADDR" => request_env.headers.delete("REMOTE_ADDR"),
        "rack.url_scheme" => uri.scheme,
        # AnyCable specific fields
        "anycable.raw_cstate" => request_env.cstate&.to_h
      )

      env.merge!(build_headers(request_env.headers))
    end

    def base_rack_env
      # Minimum required variables according to Rack Spec
      # (not all of them though, just those enough for Action Cable to work)
      # See https://rubydoc.info/github/rack/rack/master/file/SPEC
      {
        "REQUEST_METHOD" => "GET",
        "SCRIPT_NAME" => "",
        "PATH_INFO" => "/",
        "QUERY_STRING" => "",
        "SERVER_NAME" => "",
        "SERVER_PORT" => "80",
        "rack.url_scheme" => "http",
        "rack.input" => ""
      }
    end

    def build_socket(**options)
      AnyCable::Socket.new(**options)
    end

    def build_headers(headers)
      headers.each_with_object({}) do |(k, v), obj|
        k = k.upcase
        k.tr!("-", "_")
        obj["HTTP_#{k}"] = v
      end
    end

    def build_env_response(socket)
      AnyCable::EnvResponse.new(
        cstate: socket.cstate.changed_fields
      )
    end

    def logger
      AnyCable.logger
    end

    def factory
      AnyCable.connection_factory
    end

    def notify_exception(exp, method_name, message)
      AnyCable::ExceptionsHandling.notify(exp, method_name.to_s, message.to_h)
    end
  end
end
# rubocop:enable Metrics/AbcSize
# rubocop:enable Metrics/MethodLength
# rubocop:enable Metrics/ClassLength
