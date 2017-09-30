# frozen_string_literal: tru

module WithEnv
  def with_env(env)
    old_env = nil
    begin
      old_env = env.each_with_object({}) do |(k, v), obj|
        obj[k] = ENV[k] if ENV.key?(k)
        ENV[k] = v
      end
      yield
    ensure
      old_env&.each { |k, v| ENV[k] = v }
    end
  end
end
