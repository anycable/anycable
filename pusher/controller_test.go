package pusher

import (
	"context"
	"log/slog"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPusherController(t *testing.T) {
	env := common.NewSessionEnv("ws://demo.anycable.io/pusher/app-id", &map[string]string{"cookie": "val=1;"})
	conf := NewConfig()
	bro := mocks.Broker{}
	ctrl := NewController(&bro, &conf, slog.Default())

	bro.On("PresenceInfo", mock.Anything).Return(&common.PresenceInfo{
		Type:  common.PresenceInfoType,
		Total: 1,
		Records: []*common.PresenceEvent{
			{
				ID:   "a2",
				Info: map[string]interface{}{"name": "alice"},
			},
		},
	}, nil)

	t.Run("Subscribe private (success)", func(t *testing.T) {
		channel := `{"channel":"$pusher","stream":"private-party"}`

		res, err := ctrl.Subscribe(context.Background(), "1234.1234", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"private-party"}, res.Streams)
		assert.Equal(t, "private-party", res.IState[common.WHISPER_STREAM_STATE])
		assert.Equal(t, "", res.IState[common.PRESENCE_STREAM_STATE])
		assert.Equal(t, -1, res.DisconnectInterest)
	})

	t.Run("Subscribe presence (success)", func(t *testing.T) {
		channel := `{"channel":"$pusher","stream":"presence-room"}`

		env.MergeChannelState(channel, &map[string]string{
			"channel_data": `{"user_id":"b4","user_info":{"name":"bob"}}`,
		})

		res, err := ctrl.Subscribe(context.Background(), "1234.1234", env, "name=jack", channel)

		confirmationMsg := &common.Reply{
			Type:       common.ConfirmedType,
			Identifier: channel,
			Message:    `{"presence":{"count":2,"ids":["a2","b4"],"hash":{"a2":{"name":"alice"},"b4":{"name":"bob"}}}}`,
		}
		confirmation := string(utils.ToJSON(confirmationMsg))

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{confirmation}, res.Transmissions)
		assert.Equal(t, []string{"presence-room"}, res.Streams)
		assert.Equal(t, "presence-room", res.IState[common.WHISPER_STREAM_STATE])
		assert.Equal(t, "presence-room", res.IState[common.PRESENCE_STREAM_STATE])
		assert.Equal(t, -1, res.DisconnectInterest)
		assert.Equal(t, "b4", res.Presence.ID)
		assert.Equal(t, common.PresenceJoinType, res.Presence.Type)
		assert.Equal(t, map[string]interface{}{"name": "bob"}, res.Presence.Info)
	})

	t.Run("Subscribe private (failure)", func(t *testing.T) {
		channel := `{"channel":"$pusher","stream":""}`

		res, err := ctrl.Subscribe(context.Background(), "1234.1234", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(channel)}, res.Transmissions)
	})

	t.Run("Subscribe public (success)", func(t *testing.T) {
		channel := `{"channel":"$pusher","stream":"all-chat"}`

		res, err := ctrl.Subscribe(context.Background(), "1234.1234", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"all-chat"}, res.Streams)
		assert.Nil(t, res.IState)
		assert.Equal(t, -1, res.DisconnectInterest)
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		channel := `{"channel":"$pusher","stream":"private-party"}`

		res, err := ctrl.Unsubscribe(context.Background(), "1234.1234", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{}, res.Transmissions)
		assert.Equal(t, []string{}, res.Streams)
		assert.Equal(t, true, res.StopAllStreams)
	})
}
