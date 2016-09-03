module ApplicationCable
  class Connection < ActionCable::Connection::Base
    identified_by :current_user
    identified_by :url

    def connect
      self.current_user = verify_user
      self.url = request.url if current_user
    end

    private

    def verify_user
      return reject_unauthorized_connection unless cookies[:username].present?
      User.new(name: cookies[:username], secret: request.params[:token])
    end
  end
end
