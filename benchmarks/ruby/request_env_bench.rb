# frozen_string_literal: true

require "benchmark_driver"

# This is the benchmark to measure the impact of promoting some HTTP headers
# to Env fields. That should reduce the payload size as well as improve the rack env building
# speed.

protobuf = <<~CODE
  require "google/protobuf"

  Google::Protobuf::DescriptorPool.generated_pool.build do
    add_file("rpc-bench.proto", syntax: :proto3) do
      add_message "anycable_bench.Env" do
        optional :path, :string, 1
        map :headers, :string, :string, 2
      end
    end

    add_file("rpc-bench.proto", syntax: :proto3) do
      add_message "anycable_bench.FlatEnv" do
        optional :path, :string, 1
        optional :query, :string, 2
        optional :host, :string, 3
        optional :port, :string, 4
        optional :scheme, :string, 5
        optional :cookie, :string, 6
        optional :origin, :string, 7
        optional :remote_addr, :string, 8
        map :headers, :string, :string, 9
      end
    end
  end

  Env = Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable_bench.Env").msgclass
  FlatEnv = Google::Protobuf::DescriptorPool.generated_pool.lookup("anycable_bench.FlatEnv").msgclass

  message = Env.new(
    path: "/cable?token=secret",
    headers: {
      "cookie" => "session_id=123456;",
      "remote_addr" => "192.1.1.42",
      "origin" => "anycable.io",
      "x-api-token" => "api-token-2020"
    }
  )

  payload = Env.encode(message)

  message2 = FlatEnv.new(
    path: "/cable",
    query: "?token=secret",
    port: "80",
    scheme: "ws://",
    host: "ws.anycable.io",
    cookie: "session_id=123456;",
    origin: "anycable.io",
    remote_addr: "192.1.1.42",
    headers: {
      "x-api-token" => "api-token-2020"
    }
  )

  payload2 = FlatEnv.encode(message2)

  def build_rack(rpc_env)
    uri = URI.parse(rpc_env.path)

    env = {}
    env.merge!(
      "PATH_INFO" => uri.path,
      "QUERY_STRING" => uri.query,
      "SERVER_NAME" => uri.host,
      "SERVER_PORT" => uri.port.to_s,
      "HTTP_HOST" => uri.host,
      "REMOTE_ADDR" => rpc_env.headers.delete("remote_addr"),
      "rack.url_scheme" => uri.scheme
    )

    env.merge!(build_headers(rpc_env.headers))
  end

  def build_rack2(rpc_env)
    env = {}
    env.merge!(
      "PATH_INFO" => rpc_env.path,
      "QUERY_STRING" => rpc_env.query,
      "SERVER_NAME" => rpc_env.host,
      "SERVER_PORT" => rpc_env.port,
      "HTTP_HOST" => rpc_env.host,
      "REMOTE_ADDR" => rpc_env.remote_addr,
      "rack.url_scheme" => rpc_env.scheme
    )

    env.merge!(build_headers(rpc_env.headers))
  end

  def build_headers(headers)
    headers.each_with_object({}) do |(k, v), obj|
      k = k.upcase
      k.tr!("-", "_")
      obj["HTTP_\#{k}"] = v
    end
  end
CODE

Benchmark.driver do |x|
  x.prelude %(
    #{protobuf}
  )

  x.report "#decode (baseline)", %{
    Env.decode(payload)
  }

  x.report "#decode (flatten)", %{
    FlatEnv.decode(payload2)
  }
end

Benchmark.driver do |x|
  x.prelude %(
    #{protobuf}
  )

  x.report "#build_rack (baseline)", %(
    build_rack message
  )

  x.report "#build_rack (flatten)", %(
    build_rack2 message2
  )
end

Benchmark.driver do |x|
  x.prelude %(
    #{protobuf}
  )

  x.report "#decode + #build_rack (baseline)", %{
    build_rack Env.decode(payload)
  }

  x.report "#decode + #build_rack (flatten)", %{
    build_rack2 FlatEnv.decode(payload2)
  }
end
