package metrics

type NoopMetrics struct {
}

func (NoopMetrics) CounterIncrement(name string) {
}

func (NoopMetrics) CounterAdd(name string, val uint64) {
}
