package metrics

type NoopMetrics struct {
}

func (NoopMetrics) CounterIncrement(name string) {
}

func (NoopMetrics) CounterAdd(name string, val uint64) {
}

func (NoopMetrics) GaugeSet(name string, val uint64) {
}

func (NoopMetrics) GaugeIncrement(name string) {
}

func (NoopMetrics) GaugeDecrement(name string) {
}

func (NoopMetrics) RegisterCounter(name string, desc string) {
}

func (NoopMetrics) RegisterGauge(name string, desc string) {
}

var _ Instrumenter = (*NoopMetrics)(nil)
