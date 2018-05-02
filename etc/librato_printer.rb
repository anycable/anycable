module MetricsFormatter
  KEYS = %w(clients_num goroutines_num)

  def self.call(data)
    parts = []

    data.each do |key, value|
      parts << "sample##{key}=#{value}" if KEYS.include?(key)
    end

    parts.join(' ')
  end
end
