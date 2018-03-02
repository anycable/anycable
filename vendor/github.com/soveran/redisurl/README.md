redisurl
========

Connect to Redis using a `REDIS_URL`.

Usage
=====

It uses [Redigo][redigo] under the hood:

```go
import "redisurl"

// Connect using os.Getenv("REDIS_URL").
c, err := redisurl.Connect()

// Alternatively, connect using a custom Redis URL.
c, err := redisurl.ConnectToURL("redis://...")
```

In both cases you will get the result values of calling
`redis.Dial(...)`, that is, an instance of `redis.Conn` and an
error.

[redigo]: https://github.com/garyburd/redigo

Installation
============

Install it using the "go get" command:

    go get github.com/soveran/redisurl


