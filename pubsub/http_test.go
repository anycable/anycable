package pubsub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
)

func TestHttpHandler(t *testing.T) {
	handler := &mocks.Handler{}
	config := HTTPConfig{}
	secretConfig := HTTPConfig{Secret: "secret"}
	subscriber := NewHTTPSubscriber(handler, &config)
	protectedSubscriber := NewHTTPSubscriber(handler, &secretConfig)

	payload, err := json.Marshal(map[string]string{"stream": "any_test", "data": "123_test"})
	if err != nil {
		t.Fatal(err)
	}

	handler.On(
		"HandlePubSub",
		payload,
	)

	t.Run("Handles broadcasts", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(subscriber.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Rejects non-POST requests", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", strings.NewReader(string(payload)))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(subscriber.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("Rejects when authorization header is missing", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(protectedSubscriber.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Rejects when authorization header is valid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		req.Header.Set("Authorization", "Bearer secret")

		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(protectedSubscriber.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}
