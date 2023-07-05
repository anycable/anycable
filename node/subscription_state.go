package node

import "sync"

type SubscriptionState struct {
	channels map[string]map[string]struct{}
	mu       sync.RWMutex
}

func NewSubscriptionState() *SubscriptionState {
	return &SubscriptionState{channels: make(map[string]map[string]struct{})}
}

func (st *SubscriptionState) HasChannel(id string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()

	_, ok := st.channels[id]
	return ok
}

func (st *SubscriptionState) AddChannel(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.channels[id] = make(map[string]struct{})
}

func (st *SubscriptionState) RemoveChannel(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.channels, id)
}

func (st *SubscriptionState) Channels() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	keys := make([]string, len(st.channels))
	i := 0

	for k := range st.channels {
		keys[i] = k
		i++
	}
	return keys
}

func (st *SubscriptionState) ToMap() map[string][]string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	res := make(map[string][]string, len(st.channels))

	for k, v := range st.channels {
		streams := make([]string, len(v))

		i := 0
		for name := range v {
			streams[i] = name
			i++
		}

		res[k] = streams
	}

	return res
}

func (st *SubscriptionState) AddChannelStream(id string, stream string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.channels[id]; ok {
		st.channels[id][stream] = struct{}{}
	}
}

func (st *SubscriptionState) RemoveChannelStream(id string, stream string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.channels[id]; ok {
		delete(st.channels[id], stream)
	}
}

func (st *SubscriptionState) RemoveChannelStreams(id string) []string {
	st.mu.Lock()
	defer st.mu.Unlock()

	if streamNames, ok := st.channels[id]; ok {
		st.channels[id] = make(map[string]struct{})

		streams := make([]string, len(streamNames))

		i := 0
		for key := range streamNames {
			streams[i] = key
			i++
		}

		return streams
	}

	return nil
}

func (st *SubscriptionState) StreamsFor(id string) []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if streamNames, ok := st.channels[id]; ok {
		streams := make([]string, len(streamNames))

		i := 0
		for key := range streamNames {
			streams[i] = key
			i++
		}

		return streams
	}

	return nil
}
