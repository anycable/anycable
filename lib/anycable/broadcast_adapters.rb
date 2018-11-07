# frozen_string_literal: true

module AnyCable
  module BroadcastAdapters # :nodoc:
    module_function

    # rubocop: disable Metrics/AbcSize, Metrics/MethodLength
    def lookup_adapter(args)
      adapter, options = Array(args)
      path_to_adapter = "anycable/broadcast_adapters/#{adapter}"
      adapter_class_name = adapter.to_s.split("_").map(&:capitalize).join

      unless BroadcastAdapters.const_defined?(adapter_class_name, false)
        begin
          require path_to_adapter
        rescue LoadError => e
          # We couldn't require the adapter itself.
          if e.path == path_to_adapter
            raise e.class, "Couldn't load the '#{adapter}' broadcast adapter for AnyCable",
                  e.backtrace
          # Bubbled up from the adapter require.
          else
            raise e.class, "Error loading the '#{adapter}' broadcast adapter for AnyCable",
                  e.backtrace
          end
        end
      end

      BroadcastAdapters.const_get(adapter_class_name, false).new(options || {})
    end
    # rubocop: enable Metrics/AbcSize, Metrics/MethodLength
  end
end
