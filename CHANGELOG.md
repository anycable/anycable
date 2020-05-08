# Change log

## 🚧 1.0.0 (_coming soon_)

- Add `:http` broadcasting adapter. ([@palkan][])

- **RPC schema has changed**. ([@palkan][])

Using `anycable-go` v1.x is required.

- **Ruby 2.5+ is required**. ([@palkan][])

- Added RPC proto version check. ([@palkan][])

Server must sent `protov` metadata with the supported versions (comma-separated list). If there is no matching version an exception is raised.

Current RPC proto version is **v1**.

- Added `request` support to channels. ([@palkan][])

Now you can access `request` object in channels, too (e.g., to read headers/cookies/URL/etc).

- Change default server address from `[::]:50051` to `127.0.0.1:50051`. ([@palkan][])

See [#71](https://github.com/anycable/anycable/pull/71).

- Fix building Redis Sentinel config. ([@palkan][])

---

See [Changelog](https://github.com/anycable/anycable/blob/0-6-stable/CHANGELOG.md) for versions <1.0.0.

[@palkan]: https://github.com/palkan
[@sponomarev]: https://github.com/sponomarev
[@bibendi]: https://github.com/bibendi
