module ApplicationCable
  class Channel < ActionCable::Channel::Base
    after_subscribe -> { log_event('subscribed') }

    after_unsubscribe -> { log_event('unsubscribed') }

    private

    def log_event(type)
      ApplicationCable::Connection.log_event(
        identifier, { type: type, user: current_user.name }
      )
    end
  end
end
