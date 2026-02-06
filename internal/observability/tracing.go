package observability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TraceID string
type SpanID string

type SpanContext struct {
	TraceID TraceID
	SpanID  SpanID
	Parent  *SpanID
}

type Span struct {
	ctx        *SpanContext
	name       string
	startTime  time.Time
	endTime    *time.Time
	attributes map[string]any
	events     []SpanEvent
	status     SpanStatus
	mu         sync.RWMutex
}

type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]any
}

type SpanStatus struct {
	Code    StatusCode
	Message string
}

type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

type Tracer struct {
	spans sync.Map
	mu    sync.RWMutex
}

func NewTracer() *Tracer {
	return &Tracer{}
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (*Span, context.Context) {
	traceID := TraceID(fmt.Sprintf("trace-%d", time.Now().UnixNano()))
	spanID := SpanID(fmt.Sprintf("span-%d", time.Now().UnixNano()))

	var parentSpanID *SpanID
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		traceID = parentSpan.ctx.TraceID
		parentSpanID = &parentSpan.ctx.SpanID
	}

	span := &Span{
		ctx: &SpanContext{
			TraceID: traceID,
			SpanID:  spanID,
			Parent:  parentSpanID,
		},
		name:       name,
		startTime:  time.Now(),
		attributes: make(map[string]any),
		events:     make([]SpanEvent, 0),
		status: SpanStatus{
			Code: StatusCodeUnset,
		},
	}

	t.spans.Store(spanID, span)

	newCtx := context.WithValue(ctx, spanContextKey{}, span)
	return span, newCtx
}

func (s *Span) SetAttribute(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attributes[key] = value
}

func (s *Span) SetAttributes(attrs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range attrs {
		s.attributes[k] = v
	}
}

func (s *Span) AddEvent(name string, attrs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event := SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	}
	s.events = append(s.events, event)
}

func (s *Span) SetStatus(code StatusCode, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = SpanStatus{
		Code:    code,
		Message: message,
	}
}

func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.endTime = &now

	if s.status.Code == StatusCodeUnset {
		s.status.Code = StatusCodeOK
	}
}

func (s *Span) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.endTime == nil {
		return time.Since(s.startTime)
	}
	return s.endTime.Sub(s.startTime)
}

func (s *Span) Export() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export := map[string]any{
		"trace_id":   string(s.ctx.TraceID),
		"span_id":    string(s.ctx.SpanID),
		"name":       s.name,
		"start_time": s.startTime.Format(time.RFC3339Nano),
		"duration":   s.Duration().Milliseconds(),
		"attributes": s.attributes,
		"status": map[string]any{
			"code":    s.status.Code,
			"message": s.status.Message,
		},
	}

	if s.ctx.Parent != nil {
		export["parent_span_id"] = string(*s.ctx.Parent)
	}

	if len(s.events) > 0 {
		events := make([]map[string]any, len(s.events))
		for i, event := range s.events {
			events[i] = map[string]any{
				"name":       event.Name,
				"timestamp":  event.Timestamp.Format(time.RFC3339Nano),
				"attributes": event.Attributes,
			}
		}
		export["events"] = events
	}

	return export
}

type spanContextKey struct{}

func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return span
	}
	return nil
}

func (t *Tracer) GetSpan(spanID SpanID) *Span {
	if span, ok := t.spans.Load(spanID); ok {
		return span.(*Span)
	}
	return nil
}

func (t *Tracer) ExportAll() []map[string]any {
	var exports []map[string]any

	t.spans.Range(func(key, value any) bool {
		span := value.(*Span)
		exports = append(exports, span.Export())
		return true
	})

	return exports
}

var globalTracer = NewTracer()

func GetTracer() *Tracer {
	return globalTracer
}
