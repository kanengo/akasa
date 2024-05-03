package akasar

import (
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/kanengo/akasar/runtime/metrics"
)

type httpLabels struct {
	Label string
	Host  string
}

type httpErrorLabels struct {
	Label string
	Host  string
	Code  int
}

var (
	httpRequestCounts        = metrics.NewCounterMap[httpLabels]("serviceakasar_http_request_count")
	httpRequestErrors        = metrics.NewCounterMap[httpErrorLabels]("serviceakasar_http_error_count")
	httpRequestLatencyMicros = metrics.NewHistogramMap[httpLabels]("serviceakasar_http_request_latency_micros", metrics.NonNegativeBuckets)
	httpRequestBytesReceived = metrics.NewHistogramMap[httpLabels]("serviceakasar_http_request_bytes_received", metrics.NonNegativeBuckets)
	httpRequestBytesRespond  = metrics.NewHistogramMap[httpLabels]("serviceakasar_http_request_bytes_respond", metrics.NonNegativeBuckets)
)

func InstrumentHandler(label string, handler http.Handler) http.Handler {
	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()

		labels := httpLabels{Label: label, Host: r.Host}
		httpRequestCounts.Get(labels).Add(1)
		defer func() {
			httpRequestLatencyMicros.Get(labels).Put(float64(time.Since(start).Microseconds()))
		}()

		if size, ok := requestSize(r); ok {
			httpRequestBytesReceived.Get(labels).Put(float64(size))
		}

		writer := instrumentResponseWriter{w: rw}

		handler.ServeHTTP(&writer, r)
		if writer.statusCode >= http.StatusBadRequest && writer.statusCode < 600 {
			httpRequestErrors.Get(httpErrorLabels{
				Label: label,
				Host:  r.Host,
				Code:  writer.statusCode,
			}).Add(1)
		}
		httpRequestBytesRespond.Get(labels).Put(float64(writer.responseSize(r)))
	})

	const traceSampleInterval = 1 * time.Second
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := newTraceSampler(traceSampleInterval, rng)
	return otelhttp.NewHandler(h, label, otelhttp.WithFilter(func(request *http.Request) bool {
		return s.shouldTrace(time.Now())
	}))
}

func InstrumentHandlerFunc(label string, f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return InstrumentHandler(label, http.HandlerFunc(f))
}

// traceSampler 是一个基于时间跨度请求的链路采样器
// 这允许每个时间间隔最多有一个请求被追踪
type traceSampler struct {
	intervalNs float64

	mu               sync.Mutex
	rng              *rand.Rand
	nextSampleTimeNs atomic.Int64
}

func newTraceSampler(interval time.Duration, rng *rand.Rand) *traceSampler {
	return &traceSampler{
		intervalNs: float64(interval / time.Nanosecond),
		rng:        rng,
	}
}

func (s *traceSampler) shouldTrace(now time.Time) bool {
	nowNs := now.UnixNano()

	if nowNs < s.nextSampleTimeNs.Load() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if nowNs < s.nextSampleTimeNs.Load() {
		return false
	}

	// [0, 2 * s.intervalNs] 平均值s.intervalNs
	s.nextSampleTimeNs.Store(nowNs + int64(s.rng.Float64()*s.intervalNs*2))

	return true
}

type instrumentResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
	bytesSent  int
}

func (w *instrumentResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *instrumentResponseWriter) Write(bytes []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	n, err := w.w.Write(bytes)
	w.bytesSent += n

	return n, err
}

func (w *instrumentResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode == 0 {
		w.statusCode = statusCode
	}

	w.w.WriteHeader(statusCode)
}

func (w *instrumentResponseWriter) responseSize(req *http.Request) int {
	size := 0
	size += len(req.Proto) // HTTP/1.1
	size += 3              // e.g., 200
	if w.statusCode == 0 {
		size += len(http.StatusText(http.StatusOK))
	} else {
		size += len(http.StatusText(w.statusCode))
	}

	for key, values := range w.Header() {
		for _, value := range values {
			size += len(key) + len(value)
		}
	}

	size += w.bytesSent

	return size
}

var _ http.ResponseWriter = &instrumentResponseWriter{}

func requestSize(r *http.Request) (int, bool) {
	if r.ContentLength == -1 {
		return 0, false
	}

	size := 0
	size += len(r.Method)
	size += len(r.Proto)
	if r.URL != nil {
		size += len(r.URL.Path)
		size += len(r.URL.RawQuery)
		size += len(r.Host)
	}

	for key, values := range r.Header {
		for _, value := range values {
			size += len(key) + len(value)
		}
	}
	size += int(r.ContentLength)

	return size, true
}
