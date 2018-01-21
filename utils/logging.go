package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
)

// InitLogger sets log level, format and output
func InitLogger(format string, level string) error {
	logLevel, err := log.ParseLevel(level)

	if err != nil {
		msg := fmt.Sprintf("Unknown log level: %s.\nAvailable levels are: debug, info, warn, error, fatal", level)
		return errors.New(msg)
	}

	log.SetLevel(logLevel)

	if format == "text" {
		log.SetHandler(&LogHandler{writer: os.Stdout, tty: IsTTY()})
	} else if format == "json" {
		log.SetHandler(json.New(os.Stdout))
	} else {
		msg := fmt.Sprintf("Unknown log format: %s.\nAvaialable formats are: text, json", format)
		return errors.New(msg)
	}

	return nil
}
