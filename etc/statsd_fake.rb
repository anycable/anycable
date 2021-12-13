# frozen_string_literal: true

require "socket"

Socket.udp_server_loop(8045) do |data, src|
  puts data
end
