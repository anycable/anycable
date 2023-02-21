require "bundler/inline"

retried = false
begin
  gemfile(retried, quiet: ENV["LOG"] != "1") do
    source "https://rubygems.org"

    gem "async-dns"
  end
rescue Gem::MissingSpecError, Bundler::GemNotFound
  raise if retried

  retried = true
  retry
end

require 'async/dns'

IN = Resolv::DNS::Resource::IN

RPC_ADDRESSES = %w[
  127.0.0.1
  127.0.0.2
  127.0.0.3
  127.0.0.4
]

class Server < Async::DNS::Server
	def process(name, resource_class, transaction)
    if resource_class == IN::A && name.match?(%r{anycable-rpc.local})
      addresses = alive_rpcs
      if addresses.empty?
        return transaction.fail!(:NXDomain)
      end

      transaction.append_question!
      addresses.each do
        transaction.add([IN::A.new(_1)], {})
      end

      @logger.info "Resolved RPCs: #{addresses}"

      return
    end

		transaction.fail!(:NXDomain)
	end

  private

  def alive_rpcs
    RPC_ADDRESSES.select do
      Socket.tcp(_1, 50051, connect_timeout: 0.1).close
      true
    rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, Errno::ETIMEDOUT, SocketError
      false
    end
  end
end

server = Server.new([[:udp, '127.0.0.1', 2346], [:tcp, '127.0.0.1', 2346]])
server.run
