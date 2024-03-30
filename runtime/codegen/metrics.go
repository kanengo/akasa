package codegen

import (
	"time"

	"github.com/kanengo/akasar/runtime/metrics"
)

const (
	MethodCountsName       = "service_akasar_method_count"
	MethodErrorCountsName  = "service_akasar_method_error_count"
	MethodLatenciesName    = "service_akasar_method_latency_micros"
	MethodBytesRequestName = "service_akasar_method_bytes_request"
	MethodBytesReplyName   = "service_akasar_method_bytes_reply"
)

var (
	methodCounts = metrics.NewCounterMap[MethodLabels](
		MethodCountsName,
	)
	methodErrors = metrics.NewCounterMap[MethodLabels](
		MethodErrorCountsName,
	)
	methodLatencies = metrics.NewHistogramMap[MethodLabels](
		MethodLatenciesName,
		metrics.NonNegativeBuckets,
	)
	methodBytesRequest = metrics.NewHistogramMap[MethodLabels](
		MethodBytesRequestName,
		metrics.NonNegativeBuckets,
	)
	methodBytesReply = metrics.NewHistogramMap[MethodLabels](
		MethodBytesReplyName,
		metrics.NonNegativeBuckets,
	)
)

type MethodLabels struct {
	Caller    string //调用方组件全名
	Component string //组件全名
	Method    string // 方法名
	Remote    bool   //是否远程调用
}

type MethodMetrics struct {
	remote       bool
	count        *metrics.Counter
	errCount     *metrics.Counter
	latency      *metrics.Histogram
	bytesRequest *metrics.Histogram
	bytesReply   *metrics.Histogram
}

func MethodMetricsFor(labels MethodLabels) *MethodMetrics {
	return &MethodMetrics{
		remote:       labels.Remote,
		count:        methodCounts.Get(labels),
		errCount:     methodErrors.Get(labels),
		latency:      methodLatencies.Get(labels),
		bytesRequest: methodBytesRequest.Get(labels),
		bytesReply:   methodBytesReply.Get(labels),
	}
}

type MethodCallHandle struct {
	start time.Time
}

func (m *MethodMetrics) Begin() MethodCallHandle {
	return MethodCallHandle{time.Now()}
}

func (m *MethodMetrics) End(h MethodCallHandle, failed bool, requestBytes, replyBytes int) {
	latency := time.Since(h.start).Microseconds()
	m.count.Inc()
	if failed {
		m.errCount.Inc()
	}
	m.latency.Put(float64(latency))
	if m.remote {
		m.bytesRequest.Put(float64(requestBytes))
		m.bytesReply.Put(float64(replyBytes))
	}
}
