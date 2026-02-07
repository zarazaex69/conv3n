package engine

import (
	"bufio" // For reading lines from stdout
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"      // For os.Stat to check file existence
	"os/exec" // For running Bun processes
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zarazaex69/conv3n/internal/storage"
)

// TriggerType defines the type of trigger
type TriggerType string

const (
	TriggerTypeCron     TriggerType = "cron"
	TriggerTypeInterval TriggerType = "interval"
	TriggerTypeWebhook  TriggerType = "webhook"
	TriggerTypeTS       TriggerType = "typescript" // New type for TypeScript-based triggers
)

// Trigger represents a workflow trigger configuration
type Trigger struct {
	ID         string                 `json:"id"`
	WorkflowID string                 `json:"workflow_id"`
	Type       TriggerType            `json:"type"`
	Config     map[string]interface{} `json:"config"`
	Enabled    bool                   `json:"enabled"`
	FilePath   string                 `json:"file_path"` // Path to the TypeScript trigger file
}

// TriggerRunner interface for different trigger implementations
type TriggerRunner interface {
	Start(ctx context.Context) error
	Stop() error
	ID() string
	Type() TriggerType

	Invoke(ctx context.Context, payload map[string]interface{}) error
}

// ManagerForRunner defines the interface that runners use to interact with the trigger manager.

type ManagerForRunner interface {
	Fire(ctx context.Context, triggerID string, payload map[string]interface{}) error
	GetTrigger(triggerID string) (TriggerRunner, bool)
	Register(trigger TriggerRunner) error
	Unregister(triggerID string) error
}

// TriggerManager manages all active triggers
type TriggerManager struct {
	Store      storage.Storage
	workerPool *WorkerPool
	registry   *ExecutionRegistry
	triggers   map[string]TriggerRunner
	mu         sync.RWMutex
}

// NewTriggerManager creates a new trigger manager
func NewTriggerManager(store storage.Storage, workerPool *WorkerPool, registry *ExecutionRegistry) *TriggerManager {
	return &TriggerManager{
		Store:      store,
		workerPool: workerPool,
		registry:   registry,
		triggers:   make(map[string]TriggerRunner),
	}
}

// LoadTriggers loads enabled triggers from storage and starts them
func (tm *TriggerManager) LoadTriggers(ctx context.Context) error {
	triggers, err := tm.Store.ListAllTriggers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list triggers: %w", err)
	}

	log.Printf("Loading %d triggers from storage...", len(triggers))

	for _, t := range triggers {

		var config map[string]interface{}
		if err := json.Unmarshal(t.Config, &config); err != nil {
			log.Printf("Error parsing config for trigger %s: %v", t.ID, err)
			continue
		}

		var runner TriggerRunner

		// Check if it's a TypeScript-based trigger
		if TriggerType(t.Type) == TriggerTypeTS {
			filePath, ok := config["file_path"].(string)
			if !ok || filePath == "" {
				log.Printf("Error: TypeScript trigger %s missing 'file_path' in config", t.ID)
				continue
			}
			runner = NewTSTriggerRunner(t.ID, t.WorkflowID, filePath, config, tm)
		} else {

			switch TriggerType(t.Type) {
			case TriggerTypeCron:
				schedule, ok := config["schedule"].(string)
				if !ok {
					log.Printf("Error: cron trigger %s missing schedule", t.ID)
					continue
				}
				runner = NewCronTrigger(t.ID, t.WorkflowID, schedule, tm)

			case TriggerTypeInterval:
				intervalSec, ok := config["interval"].(float64)
				if !ok {
					log.Printf("Error: interval trigger %s missing interval", t.ID)
					continue
				}
				interval := time.Duration(intervalSec) * time.Second
				runner = NewIntervalTrigger(t.ID, t.WorkflowID, interval, tm)

			case TriggerTypeWebhook:
				runner = NewWebhookTrigger(t.ID, t.WorkflowID, tm)

			default:
				log.Printf("Warning: unsupported trigger type %s for trigger %s", t.Type, t.ID)
				continue
			}
		}

		if err := tm.Register(runner); err != nil {
			log.Printf("Error registering trigger %s: %v", t.ID, err)
		}
	}

	return nil
}

// TSTriggerRunner manages a TypeScript-based trigger executed by Bun.
type TSTriggerRunner struct {
	id            string
	workflowID    string
	triggerType   TriggerType
	filePath      string
	config        map[string]interface{}
	manager       ManagerForRunner
	cmd           *exec.Cmd
	stdin         *bufio.Writer
	stdoutScanner *bufio.Scanner
	stopChan      chan struct{}
	readyChan     chan error
	requests      sync.Map // Stores channels for pending requests (event replies)
	isReady       bool
	mu            sync.Mutex // Protects write access to stdin and state changes
	cancelContext context.CancelFunc
}

// NewTSTriggerRunner creates a new TypeScript trigger runner.
func NewTSTriggerRunner(id, workflowID, filePath string, config map[string]interface{}, manager ManagerForRunner) *TSTriggerRunner {
	return &TSTriggerRunner{
		id:          id,
		workflowID:  workflowID,
		triggerType: TriggerTypeTS,
		filePath:    filePath,
		config:      config,
		manager:     manager,
		stopChan:    make(chan struct{}),
		readyChan:   make(chan error, 1), // Buffered to prevent blocking if ready before read
	}
}

func (tr *TSTriggerRunner) ID() string {
	return tr.id
}

func (tr *TSTriggerRunner) Type() TriggerType {
	return tr.triggerType
}

// Start spawns the Bun process and sets up IPC.
func (tr *TSTriggerRunner) Start(ctx context.Context) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Ensure the trigger file exists
	if _, err := os.Stat(tr.filePath); os.IsNotExist(err) {
		return fmt.Errorf("TypeScript trigger file not found: %s", tr.filePath)
	}

	// Create a child context for the Bun process that can be cancelled
	processCtx, cancel := context.WithCancel(context.Background())
	tr.cancelContext = cancel

	// Command to run the TypeScript trigger using Bun

	// with 'bun run <script.ts>' which internally calls `runTrigger`.
	tr.cmd = exec.CommandContext(processCtx, "bun", "run", tr.filePath)
	tr.cmd.Dir = "." // Run from project root, or specific triggers dir

	// Setup stdin pipe for sending messages to the Bun process
	stdinPipe, err := tr.cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stdin pipe for TS trigger %s: %w", tr.id, err)
	}
	tr.stdin = bufio.NewWriter(stdinPipe)

	// Setup stdout pipe for reading messages from the Bun process
	stdoutPipe, err := tr.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stdout pipe for TS trigger %s: %w", tr.id, err)
	}
	tr.stdoutScanner = bufio.NewScanner(stdoutPipe)

	// Start the Bun process
	if err := tr.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start Bun process for TS trigger %s: %w", tr.id, err)
	}

	log.Printf("TS trigger %s: Bun process started (PID: %d)", tr.id, tr.cmd.Process.Pid)

	// Start a goroutine to read and process messages from the Bun process's stdout
	go tr.readStdoutLoop(processCtx)

	// Send the initial 'start' message with config to the TS trigger
	startMsg := map[string]interface{}{
		"type":   "start",
		"config": tr.config,
	}
	if err := tr.sendToTS(startMsg); err != nil {
		cancel()
		return fmt.Errorf("failed to send initial start message to TS trigger %s: %w", tr.id, err)
	}

	// Wait for the TS trigger to signal that it's ready
	select {
	case err := <-tr.readyChan:
		if err != nil {
			cancel()
			return fmt.Errorf("TS trigger %s reported error during startup: %w", tr.id, err)
		}
		tr.isReady = true
		log.Printf("TS trigger %s is ready.", tr.id)
		return nil
	case <-time.After(10 * time.Second): // Timeout for startup
		cancel()
		return fmt.Errorf("TS trigger %s timed out during startup", tr.id)
	case <-ctx.Done(): // Context cancelled while waiting
		cancel()
		return ctx.Err()
	}
}

// Stop sends a kill signal to the Bun process and waits for it to exit.
func (tr *TSTriggerRunner) Stop() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.cmd == nil || tr.cmd.Process == nil {
		return fmt.Errorf("TS trigger %s is not running", tr.id)
	}

	log.Printf("TS trigger %s: Stopping Bun process (PID: %d)", tr.id, tr.cmd.Process.Pid)

	// Send a 'kill' message for graceful shutdown in the TS script
	killMsg := map[string]interface{}{"type": "kill"}
	if err := tr.sendToTS(killMsg); err != nil {
		log.Printf("TS trigger %s: failed to send kill message: %v. The process will be terminated.", tr.id, err)
	}

	// Cancel the context, which sends a signal to the process
	if tr.cancelContext != nil {
		tr.cancelContext()
	}

	// Wait for the process to exit, but with a timeout to prevent deadlocks.
	done := make(chan error, 1)
	go func() {
		done <- tr.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("TS trigger %s: Bun process exited with error: %v", tr.id, err)
		} else {
			log.Printf("TS trigger %s: Bun process exited successfully.", tr.id)
		}
	case <-time.After(5 * time.Second):
		log.Printf("TS trigger %s: timed out waiting for process to exit. Forcing kill.", tr.id)

		if err := tr.cmd.Process.Kill(); err != nil {
			log.Printf("TS trigger %s: failed to force kill process: %v", tr.id, err)
			return fmt.Errorf("failed to kill stuck trigger process %s", tr.id)
		}
	}

	// Clean up resources
	close(tr.stopChan)
	tr.cmd = nil
	tr.stdin = nil
	tr.stdoutScanner = nil
	tr.isReady = false
	tr.requests = sync.Map{} // Clear any pending requests
	return nil
}

// Invoke sends an 'invoke' message to the running TypeScript trigger.
func (tr *TSTriggerRunner) Invoke(ctx context.Context, payload map[string]interface{}) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.isReady {
		return fmt.Errorf("TS trigger %s is not ready to receive invocations", tr.id)
	}

	invokeMsg := map[string]interface{}{
		"type":    "invoke",
		"payload": payload,
	}
	return tr.sendToTS(invokeMsg)
}

// sendToTS sends a JSON message to the Bun process's stdin.
func (tr *TSTriggerRunner) sendToTS(msg interface{}) error {
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for TS trigger %s: %w", tr.id, err)
	}
	if _, err := tr.stdin.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write JSON to TS trigger %s stdin: %w", tr.id, err)
	}
	if _, err := tr.stdin.WriteString("\n"); err != nil { // Newline delimiter
		return fmt.Errorf("failed to write newline to TS trigger %s stdin: %w", tr.id, err)
	}
	return tr.stdin.Flush()
}

// readStdoutLoop continuously reads and processes messages from the Bun process's stdout.
func (tr *TSTriggerRunner) readStdoutLoop(ctx context.Context) {
	defer func() {
		log.Printf("TS trigger %s: stdout read loop exited.", tr.id)
	}()

	readTimeout := 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			log.Printf("TS trigger %s: stdout read loop cancelled by context.", tr.id)
			return
		default:
		}

		done := make(chan bool, 1)
		go func() {
			tr.stdoutScanner.Scan()
			done <- true
		}()

		select {
		case <-done:
			if err := tr.stdoutScanner.Err(); err != nil {
				log.Printf("TS trigger %s: stdout scanner error: %v", tr.id, err)
				return
			}

			line := tr.stdoutScanner.Text()
			if line == "" {
				continue
			}

			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("TS trigger %s: failed to unmarshal stdout message '%s': %v", tr.id, line, err)
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				log.Printf("TS trigger %s: received message with missing or invalid 'type' field: %v", tr.id, msg)
				continue
			}

			switch msgType {
			case "status":
				status, sOk := msg["status"].(string)
				if !sOk {
					log.Printf("TS trigger %s: received status message with missing or invalid 'status' field: %v", tr.id, msg)
					continue
				}
				if status == "ready" {
					tr.readyChan <- nil
				} else if status == "error" {
					errMsg, _ := msg["message"].(string)
					tr.readyChan <- fmt.Errorf("startup error: %s", errMsg)
				}

			case "event":

				requestId, rOk := msg["requestId"].(string)
				payload, pOk := msg["payload"].(map[string]interface{})
				if !rOk || !pOk {
					log.Printf("TS trigger %s: received event message with missing or invalid 'requestId' or 'payload': %v", tr.id, msg)
					continue
				}

				go func(reqID string, pld map[string]interface{}) {
					workflowCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					err := tr.manager.Fire(workflowCtx, tr.id, pld)

					replyMsg := map[string]interface{}{
						"type":      "reply",
						"requestId": reqID,
					}
					if err != nil {
						replyMsg["error"] = err.Error()
					}
					if sendErr := tr.sendToTS(replyMsg); sendErr != nil {
						log.Printf("TS trigger %s: failed to send reply for request %s: %v", tr.id, reqID, sendErr)
					}
				}(requestId, payload)

			case "error":
				errMsg, _ := msg["message"].(string)
				stack, _ := msg["stack"].(string)
				log.Printf("TS trigger %s: Error from Bun process: %s\n%s", tr.id, errMsg, stack)

			default:
				log.Printf("TS trigger %s: received unknown message type '%s': %v", tr.id, msgType, msg)
			}

		case <-time.After(readTimeout):
			log.Printf("TS trigger %s: read timeout after %v, checking process health", tr.id, readTimeout)
			if tr.cmd != nil && tr.cmd.ProcessState != nil && tr.cmd.ProcessState.Exited() {
				log.Printf("TS trigger %s: Bun process exited unexpectedly", tr.id)
				return
			}
		case <-ctx.Done():
			log.Printf("TS trigger %s: stdout read loop cancelled during read.", tr.id)
			return
		}
	}
}

// Register adds and starts a trigger
func (tm *TriggerManager) Register(trigger TriggerRunner) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.triggers[trigger.ID()]; exists {
		return fmt.Errorf("trigger already registered: %s", trigger.ID())
	}

	// Start the trigger
	if err := trigger.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start trigger: %w", err)
	}

	tm.triggers[trigger.ID()] = trigger
	log.Printf("Registered trigger: %s (type: %s)", trigger.ID(), trigger.Type())
	return nil
}

// Unregister stops and removes a trigger
func (tm *TriggerManager) Unregister(triggerID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	trigger, exists := tm.triggers[triggerID]
	if !exists {
		return fmt.Errorf("trigger not found: %s", triggerID)
	}

	// Stop the trigger
	if err := trigger.Stop(); err != nil {
		log.Printf("Warning: failed to stop trigger %s: %v", triggerID, err)
	}

	delete(tm.triggers, triggerID)
	log.Printf("Unregistered trigger: %s", triggerID)
	return nil
}

// GetTrigger returns a trigger by ID
func (tm *TriggerManager) GetTrigger(triggerID string) (TriggerRunner, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	trigger, exists := tm.triggers[triggerID]
	return trigger, exists
}

// ListTriggers returns all registered triggers
func (tm *TriggerManager) ListTriggers() []TriggerRunner {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	triggers := make([]TriggerRunner, 0, len(tm.triggers))
	for _, trigger := range tm.triggers {
		triggers = append(triggers, trigger)
	}
	return triggers
}

// StopAll stops all triggers (for graceful shutdown)
func (tm *TriggerManager) StopAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, trigger := range tm.triggers {
		if err := trigger.Stop(); err != nil {
			log.Printf("Warning: failed to stop trigger %s: %v", id, err)
		}
	}
	tm.triggers = make(map[string]TriggerRunner)
	log.Println("Stopped all triggers")
}

// ExecuteWorkflow executes a workflow triggered by a trigger
func (tm *TriggerManager) ExecuteWorkflow(ctx context.Context, workflowID, triggerID string) error {
	return tm.Fire(ctx, triggerID, nil)
}

// Fire executes a workflow triggered by a trigger with optional payload
func (tm *TriggerManager) Fire(ctx context.Context, triggerID string, payload map[string]interface{}) error {
	go func() {
		triggerExec := &storage.TriggerExecution{
			ID:        fmt.Sprintf("texec_%d", time.Now().UnixNano()),
			TriggerID: triggerID,
			FiredAt:   time.Now(),
			Status:    "running",
			Payload:   nil,
		}

		if payload != nil {
			payloadBytes, _ := json.Marshal(payload)
			triggerExec.Payload = payloadBytes
		}

		triggerRunner, exists := tm.GetTrigger(triggerID)
		if !exists {
			log.Printf("Trigger not found: %s", triggerID)
			return
		}

		_ = triggerRunner

		trigger, err := tm.Store.GetTrigger(ctx, triggerID)
		if err != nil {
			triggerExec.Status = "failed"
			msg := err.Error()
			triggerExec.Error = &msg
			tm.Store.CreateTriggerExecution(ctx, triggerExec)
			log.Printf("Failed to get trigger: %v", err)
			return
		}

		workflow, err := tm.Store.GetWorkflow(ctx, trigger.WorkflowID)
		if err != nil {
			triggerExec.Status = "failed"
			msg := err.Error()
			triggerExec.Error = &msg
			tm.Store.CreateTriggerExecution(ctx, triggerExec)
			log.Printf("Failed to get workflow: %v", err)
			return
		}

		// Parse workflow
		var wf Workflow
		if err := json.Unmarshal(workflow.Definition, &wf); err != nil {
			triggerExec.Status = "failed"
			msg := err.Error()
			triggerExec.Error = &msg
			tm.Store.CreateTriggerExecution(ctx, triggerExec)
			log.Printf("Failed to parse workflow: %v", err)
			return
		}

		execCtx := NewExecutionContext(wf.ID)

		if payload != nil {
			execCtx.TriggerData = payload
		}

		runner := NewWorkflowRunner(execCtx, tm.workerPool, tm.Store, tm.registry)

		execContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		log.Printf("Executing workflow %s triggered by %s", trigger.WorkflowID, triggerID)

		if err := runner.Run(execContext, wf); err != nil {
			triggerExec.Status = "failed"
			msg := err.Error()
			triggerExec.Error = &msg
			tm.Store.CreateTriggerExecution(ctx, triggerExec)
			log.Printf("Workflow execution failed: %v", err)
			return
		}

		triggerExec.Status = "success"
		tm.Store.CreateTriggerExecution(ctx, triggerExec)

		log.Printf("Workflow %s completed successfully", trigger.WorkflowID)
	}()

	return nil
}

// CronTrigger implements cron-based scheduling
type CronTrigger struct {
	id         string
	workflowID string
	schedule   string
	cron       *cron.Cron
	manager    *TriggerManager
}

// NewCronTrigger creates a new cron trigger
func NewCronTrigger(id, workflowID, schedule string, manager *TriggerManager) *CronTrigger {
	return &CronTrigger{
		id:         id,
		workflowID: workflowID,
		schedule:   schedule,
		manager:    manager,
	}
}

func (ct *CronTrigger) ID() string {
	return ct.id
}

func (ct *CronTrigger) Type() TriggerType {
	return TriggerTypeCron
}

// Invoke is not applicable for Go-native CronTrigger.
func (ct *CronTrigger) Invoke(ctx context.Context, payload map[string]interface{}) error {
	return fmt.Errorf("invoke not supported for CronTrigger")
}

func (ct *CronTrigger) Start(ctx context.Context) error {
	ct.cron = cron.New()

	// Add cron job
	_, err := ct.cron.AddFunc(ct.schedule, func() {
		log.Printf("Cron trigger fired: %s (schedule: %s)", ct.id, ct.schedule)

		// Execute workflow asynchronously
		go func() {
			if err := ct.manager.ExecuteWorkflow(context.Background(), ct.workflowID, ct.id); err != nil {
				log.Printf("Cron trigger execution failed: %v", err)
			}
		}()
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	ct.cron.Start()
	log.Printf("Cron trigger started: %s (schedule: %s)", ct.id, ct.schedule)
	return nil
}

func (ct *CronTrigger) Stop() error {
	if ct.cron != nil {
		ctx := ct.cron.Stop()
		<-ctx.Done()
		log.Printf("Cron trigger stopped: %s", ct.id)
	}
	return nil
}

// IntervalTrigger implements interval-based scheduling
type IntervalTrigger struct {
	id         string
	workflowID string
	interval   time.Duration
	ticker     *time.Ticker
	stop       chan bool
	manager    *TriggerManager
}

// NewIntervalTrigger creates a new interval trigger
func NewIntervalTrigger(id, workflowID string, interval time.Duration, manager *TriggerManager) *IntervalTrigger {
	return &IntervalTrigger{
		id:         id,
		workflowID: workflowID,
		interval:   interval,
		stop:       make(chan bool),
		manager:    manager,
	}
}

func (it *IntervalTrigger) ID() string {
	return it.id
}

func (it *IntervalTrigger) Type() TriggerType {
	return TriggerTypeInterval
}

// Invoke is not applicable for Go-native IntervalTrigger.
func (it *IntervalTrigger) Invoke(ctx context.Context, payload map[string]interface{}) error {
	return fmt.Errorf("invoke not supported for IntervalTrigger")
}

func (it *IntervalTrigger) Start(ctx context.Context) error {
	it.ticker = time.NewTicker(it.interval)

	go func() {
		for {
			select {
			case <-it.ticker.C:
				log.Printf("Interval trigger fired: %s (interval: %s)", it.id, it.interval)

				// Execute workflow asynchronously
				go func() {
					if err := it.manager.ExecuteWorkflow(context.Background(), it.workflowID, it.id); err != nil {
						log.Printf("Interval trigger execution failed: %v", err)
					}
				}()
			case <-it.stop:
				return
			}
		}
	}()

	log.Printf("Interval trigger started: %s (interval: %s)", it.id, it.interval)
	return nil
}

func (it *IntervalTrigger) Stop() error {
	if it.ticker != nil {
		it.ticker.Stop()
		close(it.stop)
		log.Printf("Interval trigger stopped: %s", it.id)
	}
	return nil
}

// WebhookTrigger implements webhook-based triggering
type WebhookTrigger struct {
	id         string
	workflowID string
	manager    *TriggerManager
}

// NewWebhookTrigger creates a new webhook trigger
func NewWebhookTrigger(id, workflowID string, manager *TriggerManager) *WebhookTrigger {
	return &WebhookTrigger{
		id:         id,
		workflowID: workflowID,
		manager:    manager,
	}
}

func (wt *WebhookTrigger) ID() string {
	return wt.id
}

func (wt *WebhookTrigger) Type() TriggerType {
	return TriggerTypeWebhook
}

// Invoke is not applicable for Go-native WebhookTrigger.
func (wt *WebhookTrigger) Invoke(ctx context.Context, payload map[string]interface{}) error {
	return fmt.Errorf("invoke not supported for WebhookTrigger (use TS trigger instead)")
}

func (wt *WebhookTrigger) Start(ctx context.Context) error {

	log.Printf("Webhook trigger started: %s (waiting for POST /api/webhooks/%s)", wt.id, wt.id)
	return nil
}

func (wt *WebhookTrigger) Stop() error {

	log.Printf("Webhook trigger stopped: %s", wt.id)
	return nil
}
