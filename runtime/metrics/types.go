package metrics

type MetricType int32

const (
	MetricTypeInvalid MetricType = iota
	MetricTypeCounter
	MetricTypeGauge
	MetricTypeHistogram
)
