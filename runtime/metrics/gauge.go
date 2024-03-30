package metrics

type Gauge struct {
	impl *Metric
}

func NewGauge(name string) *Gauge {
	return &Gauge{impl: Register(MetricTypeGauge, name, nil)}
}

func (g *Gauge) Name() string {
	return g.impl.Name()
}

func (g *Gauge) Set(val float64) {
	g.impl.Set(val)
}

func (g *Gauge) Add(delta float64) {
	g.impl.Add(delta)
}

func (g *Gauge) Sub(delta float64) {
	g.impl.Sub(delta)
}

type GaugeMap[L comparable] struct {
	impl *MetricMap[L]
}

func NewGaugeMap[L comparable](name string) *GaugeMap[L] {
	return &GaugeMap[L]{impl: RegisterMap[L](MetricTypeGauge, name, nil)}
}

func (gm *GaugeMap[L]) Name() string {
	return gm.impl.Name()
}

func (gm *GaugeMap[L]) Get(labels L) *Gauge {
	return &Gauge{gm.impl.Get(labels)}
}
