module MetricsFormatter
  def self.call(data)
    parts = []

    data.each do |key, value|
      parts << "sample##{key}=#{value}"
    end

    parts.join(' ')
  end
end
