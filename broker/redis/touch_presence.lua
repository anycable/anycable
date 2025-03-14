-- This script is responsible for keeping presence information alive.
local sessionId = KEYS[1]
local ttl = tonumber(ARGV[1])
local infoTTL = ttl * 3
-- when to expire the whole presence or user presence key
local cleanupTTL = ttl * 10

local sessionPresenceKey = "$ac:ps:" .. sessionId
redis.call('EXPIRE', sessionPresenceKey, cleanupTTL)

-- Get all stream-id pairs from the sessionPresenceKey
local streamIdPairs = redis.call("HGETALL", sessionPresenceKey)

-- Touch each stream-id pair to keep the presence alive
for i = 1, #streamIdPairs, 2 do
    local streamId = streamIdPairs[i]
    local userId = streamIdPairs[i + 1]
    local presenceKey = "$ac:p:" .. streamId
    local userPresenceKey = "$ac:pu:" .. streamId .. ":" .. userId

    redis.call('HEXPIRE', userPresenceKey, ttl, 'FIELDS', '1', sessionId)
    redis.call('EXPIRE', userPresenceKey, cleanupTTL)

    redis.call('HEXPIRE', presenceKey, infoTTL, 'FIELDS', '1', userId)
    redis.call('EXPIRE', presenceKey, cleanupTTL)
end

return true
