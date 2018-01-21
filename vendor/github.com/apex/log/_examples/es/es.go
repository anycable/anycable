package main

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/es"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
	"github.com/tj/go-elastic"
)

func main() {
	esClient := elastic.New("http://192.168.99.101:9200")
	esClient.HTTPClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	e := es.New(&es.Config{
		Client:     esClient,
		BufferSize: 100,
	})

	t := text.New(os.Stderr)

	log.SetHandler(multi.New(e, t))

	ctx := log.WithFields(log.Fields{
		"file": "something.png",
		"type": "image/png",
		"user": "tobi",
	})

	go func() {
		for range time.Tick(time.Millisecond * 200) {
			ctx.Info("upload")
			ctx.Info("upload complete")
			ctx.Warn("upload retry")
			ctx.WithError(errors.New("unauthorized")).Error("upload failed")
			ctx.Errorf("failed to upload %s", "img.png")
		}
	}()

	go func() {
		for range time.Tick(time.Millisecond * 25) {
			ctx.Info("upload")
		}
	}()

	select {}
}
