package main

import (
	"errors"
	"time"

	"github.com/apex/log"
)

func main() {
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
		ctx.Errorf("failed to upload %s", "img.png")
	}
}
