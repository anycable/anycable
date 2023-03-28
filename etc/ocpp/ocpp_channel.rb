# frozen_string_literal: true

class OCPPChannel < ApplicationCable::Channel
  def subscribed
    stream_for "ev/#{params["sn"]}"
  end

  def boot_notification(data)
    id, payload = data.values_at("id", "payload")

    logger.info "BootNotification: #{payload}"
  end

  def status_notification(data)
    id, payload = data.values_at("id", "payload")

    logger.info "Status Notification: #{payload}"
  end

  def authorize(data)
    id, payload = data.values_at("id", "payload")

    logger.info "Authorize: idTag — #{payload["idTag"]}"

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

  def error(data)
    id, code, message, details = data.values_at("id", "code", "message", "payload")
    logger.error "Error from EV: #{code} — #{message} (#{details})"
  end

  def ack(data)
    logger.info "ACK from EV: #{data["id"]} — #{data.dig("payload", "status")}"
  end

  private

  def transmit_ack(id:, **payload)
    transmit({command: :Ack, id:, payload:})
  end
end
