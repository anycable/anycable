-- This script is responsible for adding presence information to the cache.
-- The presence information is stored in a Hash: stream -> [user_id, info]
-- The user presence (unique sessions) is stored in a separate Hash (with expiring Fields): stream.user_id -> [session_id, '']
-- The session presence tracks session presence streams and IDs.
-- Returns 1 if this is a new presence, 0 — if the user is already present
local presenceKey = KEYS[1]
local userPresenceKey = KEYS[2]

local sessionId = ARGV[1]
local userId = ARGV[2]
local ttl = tonumber(ARGV[3])
local info = ARGV[4]
local infoTTL = ttl * 3
-- when to expire the whole presence or user presence key
local cleanupTTL = ttl * 10

local newPresence = redis.call('HLEN', userPresenceKey) == 0

-- Update presence information
redis.call('HSET', presenceKey, userId, info)
redis.call('HEXPIRE', presenceKey, infoTTL, 'FIELDS', '1', userId)
redis.call('EXPIRE', presenceKey, cleanupTTL)

-- Update user stream presence sessions
redis.call('HSET', userPresenceKey, sessionId, '')
redis.call('HEXPIRE', userPresenceKey, ttl, 'FIELDS', '1', sessionId)
redis.call('EXPIRE', userPresenceKey, cleanupTTL)

if newPresence then
  return 1
else
  return 0
end
