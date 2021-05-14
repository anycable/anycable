# Msgpack <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable Pro allows you to use Msgpack instead of JSON to serialize incoming and outgoing data. Using binary formats such as Msgpack bring the following benefits: faster (de)serialization and less data passing through network.

TBD (Bytes in/out comparison, Encode/Decode comparison)

## Usage

In order to initiate Msgpack-encoded connection, a client MUST use `"actioncable-v1-msgpack"` subprotocol during the connection (instead of the `"actioncable-v1-json"`).

A client MUST encode outgoing and incoming messages using Msgpack (see below for the default Action Cable JavaScript client).
