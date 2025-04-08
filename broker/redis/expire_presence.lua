local presenceKey = KEYS[1]
local userPresenceKey = KEYS[2]

local sessionId = ARGV[1]
local userId = ARGV[2]

redis.call('HDEL', userPresenceKey, sessionId)

local lastPresence = redis.call('HLEN', userPresenceKey) == 0

if lastPresence then
  redis.call('HDEL', presenceKey, userId)
  return 1
end

return 0
