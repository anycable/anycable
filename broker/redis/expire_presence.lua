local streamId = KEYS[1]
local sessionId = ARGV[1]
local userId = ARGV[2]

local presenceKey = "$ac:p:" .. streamId
local userPresenceKey = "$ac:pu:" .. streamId .. ":" .. userId

redis.call('HDEL', userPresenceKey, sessionId)

local lastPresence = redis.call('HLEN', userPresenceKey) == 0

if lastPresence then
  redis.call('HDEL', presenceKey, userId)
  return 1
end

return 0
