-- This script is responsible for removing session presence information for a given stream.
local presenceKey = KEYS[1]
local userPresenceKey = KEYS[2]
local sessionId = ARGV[1]
local userId = ARGV[2]

redis.call('HDEL', userPresenceKey, sessionId)
local remained = redis.call('HLEN', userPresenceKey)
if remained == 0 then
  redis.call('DEL', userPresenceKey)
  redis.call('HDEL', presenceKey, userId)
  return 1
end

return 0
