module MetricsFormatter
  def self.call(data)
    "[#{ENV["PRINTER_NAME"]}] Connections: #{data["clients_num"]}"
  end
end
