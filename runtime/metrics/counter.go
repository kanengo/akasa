package metrics

type Counter struct {
	impl *Metric
}

func NewCounter(name string) *Counter {
	return &Counter{impl: Register(MetricTypeCounter, name, nil)}
}

func (c *Counter) Name() string {
	return c.impl.Name()
}

func (c *Counter) Inc() {
	c.impl.Inc()
}

func (c *Counter) Add(delta float64) {
	c.impl.Add(delta)
}

type CounterMap[L comparable] struct {
	impl *MetricMap[L]
}

func NewCounterMap[L comparable](name string) *CounterMap[L] {
	return &CounterMap[L]{impl: RegisterMap[L](MetricTypeCounter, name, nil)}
}

func (cm *CounterMap[L]) Name() string {
	return cm.impl.Name()
}

func (cm *CounterMap[L]) Get(labels L) *Counter {
	return &Counter{cm.impl.Get(labels)}
}
