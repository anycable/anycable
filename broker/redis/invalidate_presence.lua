local queue = KEYS[1]
local cursor = ARGV[1]
local count = tonumber(ARGV[2])
local deadline = tonumber(ARGV[3])

local expires_found = 0
local next_cursor = cursor

while expires_found < count do
  local keys = redis.call('SCAN', next_cursor, 'MATCH', '$ac:ps:*', 'COUNT', '100')
  next_cursor = keys[1]
  local found_keys = keys[2]

  for _, key in ipairs(found_keys) do
    local expiry = redis.call('PEXPIRETIME', key)

    if expiry < deadline then
      local streamIdPairs = redis.call("HGETALL", key)

      for i = 1, #streamIdPairs, 2 do
          local stream = streamIdPairs[i]
          local user_id = streamIdPairs[i + 1]

          redis.call('XADD', queue, '*', 'key', key, 'stream', stream, 'user_id', user_id)
      end

      redis.call('DEL', key)
      expires_found = expires_found + 1

      if expires_found >= count then
        break
      end
    end
  end

  if next_cursor == "0" then
    break
  end
end

return {next_cursor, expires_found}
