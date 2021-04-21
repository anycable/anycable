# frozen_string_literal: true

require "spec_helper"
require "anycable/middlewares/check_version"

describe "version check spec" do
  include_context "rpc_command"

  before(:all) do
    chain = AnyCable::MiddlewareChain.new
    chain.use(AnyCable::Middlewares::CheckVersion.new("test-v1"))

    @server = AnyCable::GRPC::Server.new(
      host: AnyCable.config.rpc_host,
      **AnyCable.config.to_grpc_params,
      interceptors: chain.to_a
    )

    @server.start
  end

  after(:all) { @server.stop }

  let(:headers) do
    {
      "Cookie" => "username=john;"
    }
  end
  let(:request) { AnyCable::ConnectionRequest.new(env: env) }
  let(:meta) { {} }

  subject { service.connect(request, metadata: meta) }

  it "passes with a single matching version in meta" do
    meta["protov"] = "test-v1"
    expect(subject).to be_success
  end

  it "passes with multiple versions including matching" do
    meta["protov"] = "test-v0,test-v1"
    expect(subject).to be_success
  end

  it "fails without matching version" do
    meta["protov"] = "test-v0,test-v01"
    expect { subject }.to raise_error(
      ::GRPC::Internal,
      %r{Client supported versions: test-v0,test-v01}
    )
  end

  it "fails without metadata" do
    expect { subject }.to raise_error(
      ::GRPC::Internal,
      %r{Client supported versions: unknown}
    )
  end

  context "when calling health check service" do
    let(:service) do
      Grpc::Health::Checker.rpc_stub_class
        .new(AnyCable.config.rpc_host, :this_channel_is_insecure)
    end

    let(:params) { {service: "anycable.RPC"} }

    subject { service.check(Grpc::Health::V1::HealthCheckRequest.new(params)) }

    it "doesn't check RPC version" do
      expect(subject.status).to eq :SERVING
    end
  end
end
