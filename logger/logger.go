package logger

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/anycable/anycable-go/utils"
	"github.com/lmittmann/tint"
)

func InitLogger(format string, level string) (slog.Handler, error) {
	logLevel, err := parseLevel(level)

	if err != nil {
		return nil, err
	}

	var handler slog.Handler

	switch format {
	case "text":
		{
			opts := &tint.Options{
				Level:      logLevel,
				NoColor:    !utils.IsTTY(),
				TimeFormat: "2006-01-02 15:04:05.000",
			}
			handler = tint.NewHandler(os.Stdout, opts)
		}
	case "json":
		{
			handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
		}
	default:
		{
			return nil, fmt.Errorf("unknown log format: %s.\nAvaialable formats are: text, json", format)
		}
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return handler, nil
}

var LevelNames = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func parseLevel(level string) (slog.Level, error) {
	lvl, ok := LevelNames[level]
	if !ok {
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s.\nAvailable levels are: debug, info, warn, error", level)
	}

	return lvl, nil
}
