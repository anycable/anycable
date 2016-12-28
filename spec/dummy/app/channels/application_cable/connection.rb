module ApplicationCable
  class Connection < ActionCable::Connection::Base
    class << self
      def events_log
        @events_log ||= []
      end

      def log_event(source, data)
        events_log << { source: source, data: data }
      end
    end

    identified_by :current_user
    identified_by :url

    def connect
      self.current_user = verify_user
      self.url = request.url if current_user
    end

    def disconnect
      self.class.log_event('disconnect', name: current_user.name, url: url)
    end

    private

    def verify_user
      return reject_unauthorized_connection unless cookies[:username].present?
      User.new(name: cookies[:username], secret: request.params[:token])
    end
  end
end
