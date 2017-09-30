# frozen_string_literal: tru

module WithEnv
  UNDEF = Object.new

  def with_env(env)
    old_env = nil
    begin
      old_env = env.each_with_object({}) do |(k, v), obj|
        obj[k] = ENV.fetch(k, UNDEF)
        ENV[k] = v
      end
      yield
    ensure
      old_env&.each do |k, v|
        if v == UNDEF
          ENV.delete(k)
        else
          ENV[k] = v
        end
      end
    end
  end
end
