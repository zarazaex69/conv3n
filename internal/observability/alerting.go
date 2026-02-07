package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

type Alert struct {
	Level     AlertLevel
	Title     string
	Message   string
	Timestamp time.Time
	Metadata  map[string]any
}

type AlertManager struct {
	webhookURL string
	client     *http.Client
}

func NewAlertManager(webhookURL string) *AlertManager {
	return &AlertManager{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (am *AlertManager) Send(ctx context.Context, alert Alert) error {
	if am.webhookURL == "" {
		return nil
	}

	alert.Timestamp = time.Now()

	payload := map[string]any{
		"level":     string(alert.Level),
		"title":     alert.Title,
		"message":   alert.Message,
		"timestamp": alert.Timestamp.Format(time.RFC3339),
		"metadata":  alert.Metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", am.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := am.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("alert webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (am *AlertManager) ExecutionFailed(ctx context.Context, execID, workflowID string, err error) {
	alert := Alert{
		Level:   AlertLevelError,
		Title:   "Workflow Execution Failed",
		Message: fmt.Sprintf("Execution %s of workflow %s failed", execID, workflowID),
		Metadata: map[string]any{
			"execution_id": execID,
			"workflow_id":  workflowID,
			"error":        err.Error(),
		},
	}

	if sendErr := am.Send(ctx, alert); sendErr != nil {
		GetLogger().Error("failed to send alert", "error", sendErr)
	}
}

func (am *AlertManager) WorkerPoolDegraded(ctx context.Context, activeWorkers, totalWorkers int) {
	alert := Alert{
		Level:   AlertLevelWarning,
		Title:   "Worker Pool Degraded",
		Message: fmt.Sprintf("Only %d/%d workers are active", activeWorkers, totalWorkers),
		Metadata: map[string]any{
			"active_workers": activeWorkers,
			"total_workers":  totalWorkers,
		},
	}

	if sendErr := am.Send(ctx, alert); sendErr != nil {
		GetLogger().Error("failed to send alert", "error", sendErr)
	}
}

func (am *AlertManager) CircuitBreakerOpen(ctx context.Context, key string) {
	alert := Alert{
		Level:   AlertLevelWarning,
		Title:   "Circuit Breaker Opened",
		Message: fmt.Sprintf("Circuit breaker for '%s' is now open", key),
		Metadata: map[string]any{
			"circuit_breaker_key": key,
		},
	}

	if sendErr := am.Send(ctx, alert); sendErr != nil {
		GetLogger().Error("failed to send alert", "error", sendErr)
	}
}
