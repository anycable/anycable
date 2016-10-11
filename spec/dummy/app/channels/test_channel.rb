class TestChannel < ApplicationCable::Channel
  def subscribed
    if current_user.secret != '123'
      reject
    else
      stream_from "test"
    end
  end

  def follow
    stream_from "user_#{current_user.name}"
    stream_from "all"
  end

  def add(data)
    transmit result: (data['a'].to_i + data['b'].to_i)            
  end
end
