module MetricsHandler
  def self.call(data)
    parts = []

    data.each do |key, value|
      parts << "anycable.#{key}=#{value}"
    end

    "sample##{parts.join(' ')}"
  end
end
