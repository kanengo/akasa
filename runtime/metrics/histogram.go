package metrics

type Histogram struct {
	impl *Metric
}

func NewHistogram(name string, bounds []float64) *Histogram {
	return &Histogram{impl: Register(MetricTypeHistogram, name, bounds)}
}

func (h *Histogram) Name() string {
	return h.impl.Name()
}

func (h *Histogram) Put(val float64) {
	h.impl.Put(val)
}

type HistogramMap[L comparable] struct {
	impl *MetricMap[L]
}

func NewHistogramMap[L comparable](name string, bounds []float64) *HistogramMap[L] {
	return &HistogramMap[L]{impl: RegisterMap[L](MetricTypeHistogram, name, bounds)}
}

func (gm *HistogramMap[L]) Name() string {
	return gm.impl.Name()
}

func (gm *HistogramMap[L]) Get(labels L) *Histogram {
	return &Histogram{gm.impl.Get(labels)}
}
