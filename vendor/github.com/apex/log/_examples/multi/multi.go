package main

import (
	"errors"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
)

func main() {
	log.SetHandler(multi.New(
		text.New(os.Stderr),
		json.New(os.Stderr),
	))

	ctx := log.WithFields(log.Fields{
		"file": "something.png",
		"type": "image/png",
		"user": "tobi",
	})

	for range time.Tick(time.Millisecond * 200) {
		ctx.Info("upload")
		ctx.Info("upload complete")
		ctx.Warn("upload retry")
		ctx.WithError(errors.New("unauthorized")).Error("upload failed")
	}
}
