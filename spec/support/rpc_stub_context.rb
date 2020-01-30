# frozen_string_literal: true

shared_context "rpc stub" do
  before(:all) do
    @service = AnyCable::RPC::Stub.new(AnyCable.config.rpc_host, :this_channel_is_insecure)
  end

  let(:service) { @service }

  let(:url) { "/cable" }
  let(:headers) { {} }

  let(:env_pb) do
    uri = URI.parse(url)

    AnyCable::Env.new(
      path: uri.path,
      query: uri.query,
      host: uri.host,
      port: uri.port.to_s,
      scheme: uri.scheme,
      cookies: headers.delete("cookie"),
      remote_addr: headers.delete("REMOTE_ADDR"),
      origin: headers.delete("origin"),
      headers: headers
    )
  end
end
