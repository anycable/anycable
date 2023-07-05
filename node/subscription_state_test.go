package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscriptionStateChannels(t *testing.T) {
	t.Run("with different channels", func(t *testing.T) {
		subscriptions := NewSubscriptionState()

		subscriptions.AddChannel("{\"channel\":\"SystemNotificationChannel\"}")
		subscriptions.AddChannel("{\"channel\":\"DressageChannel\",\"id\":23376}")

		expected := []string{
			"{\"channel\":\"SystemNotificationChannel\"}",
			"{\"channel\":\"DressageChannel\",\"id\":23376}",
		}

		actual := subscriptions.Channels()

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})

	t.Run("with the same channel", func(t *testing.T) {
		subscriptions := NewSubscriptionState()

		subscriptions.AddChannel(
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}",
		)
		subscriptions.AddChannel(
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}",
		)

		expected := []string{
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8949069\"}",
			"{\"channel\":\"GraphqlChannel\",\"channelId\":\"165d8941e62\"}",
		}

		actual := subscriptions.Channels()

		for _, key := range expected {
			assert.Contains(t, actual, key)
		}
	})
}

func TestSubscriptionStreamsFor(t *testing.T) {
	subscriptions := NewSubscriptionState()

	subscriptions.AddChannel("chat_1")
	subscriptions.AddChannel("presence_1")

	subscriptions.AddChannelStream("chat_1", "a")
	subscriptions.AddChannelStream("chat_1", "b")
	subscriptions.AddChannelStream("presence_1", "z")

	assert.Contains(t, subscriptions.StreamsFor("chat_1"), "a")
	assert.Contains(t, subscriptions.StreamsFor("chat_1"), "b")
	assert.Equal(t, []string{"z"}, subscriptions.StreamsFor("presence_1"))

	subscriptions.RemoveChannelStreams("chat_1")
	assert.Empty(t, subscriptions.StreamsFor("chat_1"))
	assert.Equal(t, []string{"z"}, subscriptions.StreamsFor("presence_1"))

	subscriptions.AddChannelStream("presence_1", "y")
	subscriptions.RemoveChannelStream("presence_1", "z")
	subscriptions.RemoveChannelStream("presence_1", "t")
	assert.Equal(t, []string{"y"}, subscriptions.StreamsFor("presence_1"))
}
