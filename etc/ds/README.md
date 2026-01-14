# Durable Streams Test Client for AnyCable

A beautiful CLI client for testing AnyCable's Durable Streams (DS) implementation.

## Installation

```bash
npm install
```

## Usage

```bash
node index.js <stream-id> [options]
```

### Arguments

- `stream-id` - The ID of the stream to subscribe to (required)

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `-u, --url <url>` | Base URL for AnyCable server | `http://localhost:8080` |
| `-p, --path <path>` | DS endpoint path | `/ds` |
| `-m, --mode <mode>` | Read mode: `catchup`, `poll`, `sse` | `catchup` |
| `-h, --help` | Display help | |
| `-V, --version` | Display version | |

### Environment Variables

- `DS_BASE_URL` - Base URL for the DS endpoint (used as default for `--url`)
- `DS_PATH` - Path prefix for DS endpoint (used as default for `--path`)

## Modes

### Catchup Mode (default)

Interactive mode where you manually trigger each fetch by pressing Enter.

```bash
node index.js my-stream --mode catchup
```

**Behavior:**
1. Makes initial request on start (from `offset=now`)
2. Displays received messages in a formatted view
3. Waits for you to press Enter to fetch the next batch
4. Press `Ctrl+C` to exit

### Poll Mode (stub)

Automatic polling at regular intervals. *Not implemented yet.*

```bash
node index.js my-stream --mode poll
```

### SSE Mode (stub)

Server-Sent Events for real-time streaming. *Not implemented yet.*

```bash
node index.js my-stream --mode sse
```

## Examples

```bash
# Subscribe to "my-stream" on default server
node index.js my-stream

# Subscribe to "chat/room-1" on a custom server
node index.js chat/room-1 --url http://localhost:3000

# Using environment variables
DS_BASE_URL=http://localhost:3000 node index.js my-stream

# Show help
node index.js --help
```

## Example Output

```
╭──────────────────────────────────────────────────────────╮
│                                                          │
│  🔌 AnyCable Durable Streams Client                      │
│                                                          │
│  Stream:  my-stream                                      │
│  URL:     http://localhost:8080/ds/my-stream             │
│  Mode:    catchup                                        │
│                                                          │
╰──────────────────────────────────────────────────────────╯

⠋ Fetching messages...

✓ Received 2 messages (offset: 44::abc123)

┌ Message 1 ───────────────────────────────────────────────┐
│ {"type":"message","text":"Hello"}                        │
└──────────────────────────────────────────────────────────┘
┌ Message 2 ───────────────────────────────────────────────┐
│ {"type":"message","text":"World"}                        │
└──────────────────────────────────────────────────────────┘

↳ Offset: now → 44::abc123

▶ Press Enter to fetch more (Ctrl+C to exit)...

ℹ No new messages (up to date)

▶ Press Enter to fetch more (Ctrl+C to exit)...

👋 Goodbye!
```

## References

- [Durable Streams Specification](https://github.com/durable-streams/durable-streams)
- [AnyCable DS Design](../../ds/DESIGN.md)
- [@durable-streams/client Documentation](https://github.com/durable-streams/durable-streams/tree/main/packages/client)