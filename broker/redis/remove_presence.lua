-- This script is responsible for removing session presence information for a given stream.
local streamId = KEYS[1]
local sessionId = ARGV[1]

local sessionPresenceKey = "$ac:ps:" .. sessionId

-- Get all stream-id pairs from the sessionPresenceKey
local streamIdPairs = redis.call("HGETALL", sessionPresenceKey)
redis.call('DEL', sessionPresenceKey)

-- Touch each stream-id pair to keep the presence alive
for i = 1, #streamIdPairs, 2 do
    local stream = streamIdPairs[i]
    if stream == streamId then
      local userId = streamIdPairs[i + 1]
      local presenceKey = "$ac:p:" .. streamId
      local userPresenceKey = "$ac:pu:" .. streamId .. ":" .. userId

      redis.call('HDEL', userPresenceKey, sessionId)

      local remained = redis.call('HLEN', userPresenceKey)
      if remained == 0 then
        redis.call('DEL', userPresenceKey)
        redis.call('HDEL', presenceKey, userId)
        return userId
      end
    end
end

return ''
