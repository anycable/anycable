# REST API

AnyCable provides a REST API to perform actions and get information about AnyCable installation.

## Authentication

When the API secret is configured (either explicitly via `--api_secret` or derived from `--secret`), all API requests must include an `Authorization` header with a Bearer token:

```
Authorization: Bearer <api-secret>
```

### Secret derivation

If `--api_secret` is not explicitly set but `--secret` is provided, the API secret is automatically derived using HMAC-SHA256. This feature is used by AnyCable SDKs and usually you don't need to worry about. However, if you're not using any of the official SDKs, you can generate an API secret based on the private AnyCable secret as follows:

```sh
echo -n 'api-cable' | openssl dgst -sha256 -hmac '<your-secret>' | awk '{print $2}'
```

Or, for example, in Ruby:

```ruby
api_secret = OpenSSL::HMAC.hexdigest("SHA256", "<APPLICATION SECRET>", "api-cable")
```

## Endpoints

### POST /api/publish

Publish a broadcast message to connected clients.

#### Request

- **Method:** `POST`
- **Path:** `/api/publish` (or `<api_path>/publish` if custom path is configured)
- **Content-Type:** `application/json`
- **Authorization:** `Bearer <api-secret>` (when authentication is enabled)

#### Request Body

The request body must be a JSON object (or an array of objects) with the following structure:

```json
{
  "stream": "<stream-name>",
  "data": "<payload>",
  "meta": {}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `stream` | string | Yes | The name of the stream to publish to |
| `data` | string | Yes | The message payload (typically JSON-encoded) |
| `meta` | object | No | Additional metadata for the publication |

##### Meta fields

| Field | Type | Description |
|-------|------|-------------|
| `exclude_socket` | string | Client identifier (`sid` from welcome message) to exclude from recipients |

#### Response

| Status Code | Description |
|-------------|-------------|
| `201 Created` | Message published successfully |
| `401 Unauthorized` | Missing or invalid authentication |
| `422 Unprocessable Entity` | Invalid request method or malformed body |
| `501 Not Implemented` | Server failed to process the broadcast |

#### Examples

**Basic publish:**

```sh
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-secret" \
  -d '{"stream":"chat/1","data":"{\"message\":\"Hello, world!\"}"}' \
  http://localhost:8080/api/publish
```

**Publish multiple messages (_batch_):**

```sh
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-secret" \
  -d '[
    {"stream":"chat/1","data":"{\"message\":\"First\"}"},
    {"stream":"chat/2","data":"{\"message\":\"Second\"}"}
  ]' \
  http://localhost:8080/api/publish
```

### GET /presence/:stream/users

Retrieve presence information for a specific stream.

**NOTE:** This endpoint requires [presence](./presence.md) support to be enabled (available when using the Memory or Redis broker).

#### Request

- **Method:** `GET`
- **Path:** `/api/presence/:stream/users` (or `<api_path>/presence/:stream/users` if custom path is configured)
- **Authorization:** `Bearer <api-secret>` (when authentication is enabled)

#### URL Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `stream` | string | Yes | The name of the stream to get presence information for |

#### Response

| Status Code | Description |
|-------------|-------------|
| `200 OK` | Presence information retrieved successfully |
| `401 Unauthorized` | Missing or invalid authentication |
| `404 Not Found` | Invalid path format |
| `422 Unprocessable Entity` | Invalid request method (non-GET) |
| `501 Not Implemented` | Presence is not supported by the current broker |

#### Response Body

The response body is a JSON object with the following structure:

```json
{
  "type": "info",
  "total": 2,
  "records": [
    {
      "id": "user-1",
      "info": { "name": "Alice" }
    },
    {
      "id": "user-2",
      "info": { "name": "Bob" }
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"info"` |
| `total` | integer | Total number of unique presence records in the stream |
| `records` | array | List of presence records (omitted if empty) |

Each record in the `records` array contains:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique presence identifier for the user |
| `info` | object | User-defined presence information |

#### Examples

**Get presence for a stream:**

```sh
curl -X GET \
  -H "Authorization: Bearer your-api-secret" \
  http://localhost:8080/api/presence/chat%2F1/users
```

**Example response:**

```json
{
  "type": "info",
  "total": 2,
  "records": [
    {
      "id": "user-123",
      "info": { "name": "Alice", "status": "online" }
    },
    {
      "id": "user-456",
      "info": { "name": "Bob", "status": "away" }
    }
  ]
}
```

**Empty stream response:**

```json
{
  "total": 0
}
```

## Configuration

The API server can be configured using the following options:

| Option | Environment Variable | Default | Description |
|--------|---------------------|---------|-------------|
| `--api_port` | `ANYCABLE_API_PORT` | `0` | API server port. When set to `0`, the API runs on the main server port |
| `--api_path` | `ANYCABLE_API_PATH` | `/api` | Base path for API endpoints |
| `--api_secret` | `ANYCABLE_API_SECRET` | - | Secret token for API authentication (derived from the main secret if not passed) |

**IMPORTANT:** The API is not available if all of the below holds:

- No secret provided for AnyCable
- No dedicated port configured for the API server (`--api_port`)
- No public mode.

In other words, API is not available when it's not protected one way or another.

When the API is enabled, you will see a log message on startup:

```sh
INFO Handle API requests at http://localhost:8080/api (authorization required)
```

Or, if running without authentication (on a separate port or in a public mode):

```sh
INFO Handle API requests at http://localhost:8080/api (no authorization)
INFO API server is running without authentication
```

### CORS

The API supports CORS (Cross-Origin Resource Sharing) for browser-based requests. CORS headers are automatically added when the server is configured with CORS support.

Preflight `OPTIONS` requests are handled automatically and return a `200 OK` response with appropriate CORS headers.
