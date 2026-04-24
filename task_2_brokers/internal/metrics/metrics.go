package metrics

import (
	"sort"
	"sync"
	"time"
)

type ProducerMetrics struct {
	mu     sync.Mutex
	sent   int64
	errors int64
}

type ConsumerStats struct {
	Received      int64
	Errors        int64
	AvgLatencyMs  float64
	P95LatencyMs  float64
	MaxLatencyMs  float64
}

func NewProducerMetrics() *ProducerMetrics {
	return &ProducerMetrics{}
}

func (m *ProducerMetrics) RecordSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent++
}

func (m *ProducerMetrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

func (m *ProducerMetrics) Sent() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent
}

func (m *ProducerMetrics) Errors() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors
}

type ConsumerMetrics struct {
	mu        sync.Mutex
	received  int64
	errors    int64
	latencies []time.Duration
}

func NewConsumerMetrics() *ConsumerMetrics {
	return &ConsumerMetrics{
		latencies: make([]time.Duration, 0),
	}
}

func (m *ConsumerMetrics) RecordReceived() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received++
}

func (m *ConsumerMetrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

func (m *ConsumerMetrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies = append(m.latencies, d)
}

func (m *ConsumerMetrics) Stats() ConsumerStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := ConsumerStats{
		Received: m.received,
		Errors:   m.errors,
	}

	if len(m.latencies) == 0 {
		return stats
	}

	var sum time.Duration
	for _, d := range m.latencies {
		sum += d
		if float64(d.Milliseconds()) > stats.MaxLatencyMs {
			stats.MaxLatencyMs = float64(d.Milliseconds())
		}
	}
	stats.AvgLatencyMs = float64(sum.Milliseconds()) / float64(len(m.latencies))

	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	p95Index := int(float64(len(sorted)) * 0.95)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	stats.P95LatencyMs = float64(sorted[p95Index].Milliseconds())

	return stats
}