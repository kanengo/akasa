package metrics

import (
	"fmt"
	"math"
	"sync"
)

var (
	metricNamesMu sync.RWMutex
	metricNames   map[string]bool
)

// MetricMap 相同 name 和 label 格式但具体 label 的值不同的 metric 合集
type MetricMap[L comparable] struct {
	config         config
	mu             sync.Mutex
	metrics        map[L]*Metric
	labelExtractor *labelExtractor[L]
}

func Register(typ MetricType, name string, bounds []float64) *Metric {
	m := RegisterMap[struct{}](typ, name, bounds)
	return m.Get(struct{}{})
}

func RegisterMap[L comparable](typ MetricType, name string, bounds []float64) *MetricMap[L] {
	if err := typeCheckLabels[L](); err != nil {
		panic(err)
	}

	if name == "" {
		panic(fmt.Errorf("empty metric name"))
	}

	if typ == MetricTypeInvalid {
		panic(fmt.Errorf("metric %q: invalid metric type %v", name, typ))
	}

	// 检查bounds的每个元素是否是有效数字
	for _, x := range bounds {
		if math.IsNaN(x) {
			panic(fmt.Errorf("metric %q: non-ascending histogram bounds %v", name, bounds))
		}
	}

	metricNamesMu.Lock()
	defer metricNamesMu.Unlock()
	if metricNames[name] {
		panic(fmt.Errorf("metric %q already exists", name))
	}
	metricNames[name] = true

	return &MetricMap[L]{
		config: config{
			Typ:    typ,
			Name:   name,
			Labels: nil,
			Bounds: bounds,
		},
		metrics:        make(map[L]*Metric),
		labelExtractor: newLabelExtractor[L](),
	}
}

func (mm *MetricMap[L]) Name() string {
	return mm.config.Name
}

// Get 通过具体的labels值获取一个指定name和labels格式的metric
func (mm *MetricMap[L]) Get(labels L) *Metric {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if metric, ok := mm.metrics[labels]; ok {
		return metric
	}

	config := mm.config
	config.Labels = func() map[string]string {
		return mm.labelExtractor.Extract(labels)
	}
	metric := newMetric(config)
	mm.metrics[labels] = metric

	return metric
}
