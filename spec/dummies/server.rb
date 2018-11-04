# frozen_string_literal: true

require "webrick"
require "logger"

WEBrick::HTTPServer.new(
  Port: 9021,
  Logger: Logger.new(STDOUT),
  AccessLog: []
).tap do |server|
  server.mount_proc "/" do |_, res|
    res.status, res.body = 200, "OK"
  end
end.start
