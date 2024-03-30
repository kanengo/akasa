package metrics

import (
	"encoding/binary"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/kanengo/akasar/internal/unsafex"
	gonanoid "github.com/matoous/go-nanoid/v2"

	"golang.org/x/exp/maps"
)

var (
	metricMu sync.RWMutex
	metrics  []*Metric
)

type Metric struct {
	typ  MetricType
	name string

	labelsThunk func() map[string]string //延迟获取labels的值

	once   sync.Once         //初始化 id 和 labels
	id     uint64            //全局唯一 id
	labels map[string]string //通过调用 labelsThunk 得到的值
	fValue atomicFloat64     //Counter、Gauge的值， Histogram的和
	iValue atomic.Uint64     //Counter类型的整形增量

	// For histograms only:
	putCount atomic.Uint64   //每次调用 Put 都会增加
	bounds   []float64       //histogram bounds
	counts   []atomic.Uint64 // histogram counts
}

func NewMetric(name string) *Metric {
	return &Metric{name: name}
}

type MetricSnapshot struct {
	Id     uint64
	Name   string
	Typ    MetricType
	Labels map[string]string

	Value  float64
	Bounds []float64
	Counts []uint64
}

func (m *MetricSnapshot) Clone() *MetricSnapshot {
	c := *m

	c.Labels = maps.Clone(m.Labels)
	c.Bounds = slices.Clone(m.Bounds)
	c.Counts = slices.Clone(m.Counts)

	return &c
}

func (m *Metric) Name() string {
	return m.name
}

func (m *Metric) Inc() {
	m.iValue.Add(1)
}

func (m *Metric) Add(delta float64) {
	m.fValue.add(delta)
}

func (m *Metric) Sub(delta float64) {
	m.fValue.add(-delta)
}

func (m *Metric) Set(val float64) {
	m.fValue.set(val)
}

func (m *Metric) Put(val float64) {
	var idx int
	if len(m.bounds) == 0 || val < m.bounds[0] {

	} else {
		idx = sort.SearchFloat64s(m.bounds, val)
		if idx < len(m.bounds) && val == m.bounds[idx] {
			idx++
		}
	}
	m.counts[idx].Add(1)

	if val != 0 {
		m.fValue.add(val)
	}
	m.putCount.Add(1)
}

type config struct {
	Typ    MetricType
	Name   string
	Labels func() map[string]string
	Bounds []float64
}

func newMetric(config config) *Metric {
	metricMu.Lock()
	defer metricMu.Unlock()

	metric := &Metric{
		typ:         config.Typ,
		name:        config.Name,
		labelsThunk: config.Labels,
		bounds:      config.Bounds,
	}

	if config.Typ == MetricTypeHistogram {
		metric.counts = make([]atomic.Uint64, len(config.Bounds)+1)
	}

	metrics = append(metrics, metric)

	return metric
}

// initIdAndLabels 初始化id和labels的值.
// 通过延迟初始化到第一吃导出 metric, 避免减慢 Get() 的调用
func (m *Metric) initIdAndLabels() {
	m.once.Do(func() {
		if labels := m.labelsThunk(); len(labels) > 0 {
			m.labels = labels
		}

		// 使用8位 nanoid,转换成一个64位整型
		id := gonanoid.Must(8)
		m.id = binary.LittleEndian.Uint64(unsafex.StringToBytes(id))
	})
}

func (m *Metric) get() float64 {
	return m.fValue.get() + float64(m.iValue.Load())
}

// Snapshot 获取metric的快照
func (m *Metric) Snapshot() *MetricSnapshot {
	var counts []uint64
	if n := len(m.counts); n > 0 {
		counts = make([]uint64, n)
		for i := range m.counts {
			counts[i] = m.counts[i].Load()
		}
	}

	return &MetricSnapshot{
		Id:     m.id,
		Typ:    m.typ,
		Name:   m.name,
		Labels: maps.Clone(m.labels),
		Value:  m.get(),
		Bounds: slices.Clone(m.bounds),
		Counts: counts,
	}
}

func Snapshot() []*MetricSnapshot {
	metricMu.RLock()
	defer metricMu.RUnlock()

	snapshots := make([]*MetricSnapshot, 0, len(metrics))
	for _, metric := range metrics {
		metric.initIdAndLabels()
		snapshots = append(snapshots, metric.Snapshot())
	}

	return snapshots
}

//
