-- This script is responsible for keeping presence information alive.
local presenceKey = KEYS[1]
local userPresenceKey = KEYS[2]
local sessionId = ARGV[1]
local userId = ARGV[2]
local ttl = tonumber(ARGV[3])
local infoTTL = ttl * 3
-- when to expire the whole presence or user presence key
local cleanupTTL = ttl * 10

redis.call('HEXPIRE', userPresenceKey, ttl, 'FIELDS', '1', sessionId)
redis.call('EXPIRE', userPresenceKey, cleanupTTL)

redis.call('HEXPIRE', presenceKey, infoTTL, 'FIELDS', '1', userId)
redis.call('EXPIRE', presenceKey, cleanupTTL)

return true
