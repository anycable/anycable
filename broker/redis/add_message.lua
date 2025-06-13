-- This script is responsible for adding a message to the cache.
-- A separate key is used to store the current position in the stream.
-- A separate stream is used to store timestamps for positions (used by HistorySince).
local streamKey = KEYS[1]
local posKey = KEYS[2]
local tsKey = KEYS[3]
local epochKey = KEYS[4]
local payload = ARGV[1]
local historyLimit = ARGV[2]
local historyTTL = ARGV[3]

local pos = redis.call("incr", posKey)
local posTTL = tonumber(historyTTL) * 10
redis.call("expire", posKey, posTTL)

local epochTTL = tonumber(historyTTL) * 10
redis.call("expire", epochKey, epochTTL)

local maxlen = historyLimit

redis.call("xadd", streamKey, "MAXLEN", maxlen, pos, "d", payload)
redis.call("xadd", tsKey, "MAXLEN", maxlen, "*", "pos", pos)

redis.call("expire", streamKey, historyTTL)
redis.call("expire", tsKey, historyTTL)

return pos
