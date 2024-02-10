package erreport

import (
	"context"
	"errors"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockHandler struct {
	mock.Mock
	slog.Handler
}

func (m *MockHandler) Handle(ctx context.Context, record slog.Record) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	args := m.Called(ctx, level)
	return args.Bool(0)
}

func (m *MockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	args := m.Called(attrs)
	return args.Get(0).(slog.Handler)
}

func (m *MockHandler) WithGroup(name string) slog.Handler {
	args := m.Called(name)
	return args.Get(0).(slog.Handler)
}

type MockReporter struct {
	mock.Mock
	Reporter
}

func (m *MockReporter) CaptureException(err error) error {
	m.Called(err)
	return nil
}

func TestLogHandler_Handle(t *testing.T) {
	mockHandler := new(MockHandler)
	mockReporter := new(MockReporter)
	handler := NewLogHandler(mockHandler, mockReporter)

	err := errors.New("test error")
	record := slog.Record{
		Level: slog.LevelError,
	}

	record.AddAttrs(slog.Attr{Key: "error", Value: slog.AnyValue(err)})

	mockHandler.On("Handle", mock.Anything, record).Return(nil)
	mockReporter.On("CaptureException", err)

	err = handler.Handle(context.Background(), record)

	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
	mockReporter.AssertExpectations(t)
}

func TestLogHandler_Handle_When_No_Error_Attribute(t *testing.T) {
	mockHandler := new(MockHandler)
	mockReporter := new(MockReporter)
	handler := NewLogHandler(mockHandler, mockReporter)

	err := errors.New("test error")
	record := slog.Record{
		Level: slog.LevelError,
	}

	record.AddAttrs(slog.Attr{Key: "errata", Value: slog.AnyValue(err)})

	mockHandler.On("Handle", mock.Anything, record).Return(nil)
	mockReporter.On("CaptureException", err)

	err = handler.Handle(context.Background(), record)

	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
	mockReporter.AssertNumberOfCalls(t, "CaptureException", 0)
}

func TestLogHandler_Handle_When_Non_Error_Level(t *testing.T) {
	mockHandler := new(MockHandler)
	mockReporter := new(MockReporter)
	handler := NewLogHandler(mockHandler, mockReporter)

	err := errors.New("test error")
	record := slog.Record{
		Level: slog.LevelWarn,
	}

	record.AddAttrs(slog.Attr{Key: "error", Value: slog.AnyValue(err)})

	mockHandler.On("Handle", mock.Anything, record).Return(nil)
	mockReporter.On("CaptureException", err)

	err = handler.Handle(context.Background(), record)

	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
	mockReporter.AssertNumberOfCalls(t, "CaptureException", 0)
}

func TestLogHandler_Enabled(t *testing.T) {
	mockHandler := new(MockHandler)
	handler := NewLogHandler(mockHandler, nil)

	mockHandler.On("Enabled", mock.Anything, slog.LevelError).Return(true)

	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
	mockHandler.AssertExpectations(t)
}

func TestLogHandler_WithAttrs(t *testing.T) {
	mockHandler := new(MockHandler)
	handler := NewLogHandler(mockHandler, nil)

	attrs := []slog.Attr{{Key: "key", Value: slog.AnyValue("value")}}
	mockHandler.On("WithAttrs", attrs).Return(mockHandler)

	newHandler := handler.WithAttrs(attrs)

	assert.Equal(t, handler, newHandler)
	mockHandler.AssertExpectations(t)
}

func TestLogHandler_WithGroup(t *testing.T) {
	mockHandler := new(MockHandler)
	handler := NewLogHandler(mockHandler, nil)

	group := "group"
	mockHandler.On("WithGroup", group).Return(mockHandler)

	newHandler := handler.WithGroup(group)

	assert.Equal(t, handler, newHandler)
	mockHandler.AssertExpectations(t)
}
