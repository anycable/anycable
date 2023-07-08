# OCPP support (_alpha_)

<p class="pro-badge-header"></p>

[OCPP][] (Open Charge Point Protocol) is a communication protocol for electric vehicle charging stations. It defines a WebSocket-based RPC communication protocol to manage station and receive status updates.

AnyCable-Go Pro supports OCPP and allows you to _connect_ your charging stations to Ruby or Rails applications and control everything using Action Cable at the backend.

**NOTE:** Currently, AnyCable-Go Pro supports OCPP v1.6 only. Please, contact us if you need support for other versions.

## How it works

- EV charging station connects to AnyCable-Go via WebSocket
- The station sends a `BootNotification` request to initialize the connection
- AnyCable transforms this request into several AnyCable RPC calls to match the Action Cable interface:
  1) `Authenticate -> Connection#connect` to authenticate the station.
  2) `Command{subscribe} -> OCCPChannel#subscribed` to initialize a channel entity to association with this station.
  3) `Command{perform} -> OCCPChannel#boot_notification` to handle the `BootNotification` request.
- Subsequent requests from the station are converted into `OCCPChannel` action calls (e.g., `Authorize -> OCCPChannel#authorize`, `StartTransaction -> OCCPChannel#start_transaction`).

AnyCable also takes care of heartbeats and acknowledgment messages (unless you send them manually, see below).

## Usage

To enable OCPP support, you need to specify the `--ocpp_path` flag (or `ANYCABLE_OCPP_PATH` environment variable) specify the prefix for OCPP connections:

```sh
$ anycable-go --ocpp_path=/ocpp

...
INFO 2023-03-28T19:06:58.725Z context=main Handle OCPP v1.6 WebSocket connections at http://localhost:8080/ocpp/{station_id}
...
```

AnyCable automatically adds the `/:station_id` part to the path. You can use it to identify the station in your application.

## Example Action Cable channel class

Now, to manage EV connections at the Ruby side, you need to create a channel class. Here is an example:

```ruby
class OCPPChannel < ApplicationCable::Channel
  def subscribed
    # You can subscribe the station to its personal stream to
    # send remote comamnds to it
    # params["sn"] contains the station's serial number
    # (meterSerialNumber from the BootNotification request)
    stream_for "ev/#{params["sn"]}"
  end

  def boot_notification(data)
    # Data contains the following fields:
    #  - id - a unique message ID
    #  - command - an original command name
    #  - payload - a hash with the original request data
    id, payload = data.values_at("id", "payload")

    logger.info "BootNotification: #{payload}"

    # By default, if not ack sent, AnyCable sends the following:
    # [3, <id>, {"status": "Accepted"}]
    #
    # For boot notification response, the "interval" is also added.
  end

  def status_notification(data)
    id, payload = data.values_at("id", "payload")

    logger.info "Status Notification: #{payload}"
  end

  def authorize(data)
    id, payload = data.values_at("id", "payload")

    logger.info "Authorize: idTag — #{payload["idTag"]}"

    # For some actions, you may want to send a custom response.
    transmit_ack(id:, idTagInfo: {status: "Accepted"})
  end

  def start_transaction(data)
    id, payload = data.values_at("id", "payload")

    id_tag, connector_id = payload.values_at("idTag", "connectorId")

    logger.info "StartTransaction: idTag — #{id_tag}, connectorId — #{connector_id}"

    transmit_ack(id:, transactionId: rand(1000), idTagInfo: {status: "Accepted"})
  end

  def stop_transaction(data)
    id, payload = data.values_at("id", "payload")

    id_tag, connector_id, transaction_id = payload.values_at("idTag", "connectorId", "transactionId")

    logger.info "StopTransaction: transcationId - #{transaction_id}, idTag — #{id_tag}"

    transmit_ack(id:, idTagInfo: {status: "Accepted"})
  end

  # These are special methods to handle OCPP errors and acks
  def error(data)
    id, code, message, details = data.values_at("id", "code", "message", "payload")
    logger.error "Error from EV: #{code} — #{message} (#{details})"
  end

  def ack(data)
    logger.info "ACK from EV: #{data["id"]} — #{data.dig("payload", "status")}"
  end

  private

  def transmit_ack(id:, **payload)
    # IMPORTANT: You must use "Ack" as the command for acks,
    # so AnyCable can correctly translate them into OCPP acks.
    transmit({command: :Ack, id:, payload:})
  end
end
```

### Single-action variant

It's possible to handle all OCCP commands with a single `#receive` method at the channel class. For that, you must configure `anycable-go` to not use granular actions for OCPP:

```sh
anycable-go --ocpp_granular_actions=false

# or

ANYCABLE_OCPP_GRANULAR_ACTIONS=false anycable-go
```

In your channel class:

```ruby

class OCPPChannel < ApplicationCable::Channel
  def subscribed
    stream_for "ev/#{params["sn"]}"
  end

  def receive(data)
    id, command, payload = data.values_at("id", "command", "payload")

    logger.info "[#{id}] #{command}: #{payload}"
  end
end
```

### Remote commands

You can send remote commands to stations via Action Cable broadcasts:

```ruby
OCCPChannel.broadcast_to(
  "ev/#{serial_number}",
  {
    command: "TriggerMessage",
    id: "<uniq_id>",
    payload: {
      requestedMessage: "BootNotification"
    }
  }
)
```

[OCPP]: https://en.wikipedia.org/wiki/Open_Charge_Point_Protocol
