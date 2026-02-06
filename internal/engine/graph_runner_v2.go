package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zarazaex69/conv3n/internal/observability"
	"github.com/zarazaex69/conv3n/internal/storage"
)

type GraphRunnerV2 struct {
	workerPool      *WorkerPoolV2
	stateManager    *StateManager
	storage         storage.Storage
	registry        *ExecutionRegistry
	retryConfig     *RetryConfig
	circuitBreakers *CircuitBreakerRegistry
	metrics         *observability.Metrics
	tracer          *observability.Tracer
	logger          *observability.Logger
}

func NewGraphRunnerV2(
	ctx *ExecutionContext,
	workerPool *WorkerPoolV2,
	store storage.Storage,
	registry *ExecutionRegistry,
) *GraphRunnerV2 {
	return &GraphRunnerV2{
		workerPool:      workerPool,
		stateManager:    NewStateManager(ctx),
		storage:         store,
		registry:        registry,
		retryConfig:     DefaultRetryConfig(),
		circuitBreakers: NewCircuitBreakerRegistry(DefaultCircuitBreakerConfig()),
		metrics:         observability.GetMetrics(),
		tracer:          observability.GetTracer(),
		logger:          observability.GetLogger(),
	}
}

func (gr *GraphRunnerV2) Run(ctx context.Context, workflow Workflow) error {
	span, ctx := gr.tracer.StartSpan(ctx, "workflow.execute")
	defer span.End()

	span.SetAttributes(map[string]any{
		"workflow.id":   workflow.ID,
		"workflow.name": workflow.Name,
		"node.count":    len(workflow.Nodes),
		"edge.count":    len(workflow.Edges),
	})

	logger := gr.logger.WithWorkflow(workflow.ID, gr.stateManager.ctx.ExecutionID)
	logger.Info("workflow execution started")

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		gr.metrics.Histogram("workflow.duration", nil, nil).ObserveDuration(startTime)
		logger.Info("workflow execution completed", slog.Duration("duration", duration))
	}()

	execID, err := gr.storage.CreateExecution(ctx, workflow.ID)
	if err != nil {
		span.SetStatus(observability.StatusCodeError, err.Error())
		return fmt.Errorf("failed to create execution: %w", err)
	}

	gr.stateManager.ctx.ExecutionID = execID

	var finalStatus = storage.ExecutionStatusCompleted
	var finalError *string

	defer func() {
		stateBytes, _ := json.Marshal(gr.stateManager.ctx.Results)
		if err := gr.storage.UpdateExecutionStatus(ctx, execID, finalStatus, stateBytes, finalError); err != nil {
			logger.Error("failed to update execution status", slog.Any("error", err))
		}

		gr.metrics.Counter("workflow.executions", map[string]string{
			"status": string(finalStatus),
		}).Inc()
	}()

	startNodes := workflow.FindStartNodes()
	if len(startNodes) == 0 {
		err := fmt.Errorf("no start nodes found")
		span.SetStatus(observability.StatusCodeError, err.Error())
		return err
	}

	graph := gr.buildExecutionGraph(&workflow)

	if err := gr.executeGraph(ctx, graph, &workflow); err != nil {
		finalStatus = storage.ExecutionStatusFailed
		msg := err.Error()
		finalError = &msg
		span.SetStatus(observability.StatusCodeError, err.Error())
		return err
	}

	span.SetStatus(observability.StatusCodeOK, "")
	return nil
}

type executionNode struct {
	node         *Node
	dependencies []string
	dependents   []string
	executed     bool
	result       *BlockResult
	mu           sync.RWMutex
}

type executionGraph struct {
	nodes map[string]*executionNode
	mu    sync.RWMutex
}

func (gr *GraphRunnerV2) buildExecutionGraph(workflow *Workflow) *executionGraph {
	graph := &executionGraph{
		nodes: make(map[string]*executionNode),
	}

	for id, node := range workflow.Nodes {
		nodeCopy := node
		graph.nodes[id] = &executionNode{
			node:         &nodeCopy,
			dependencies: make([]string, 0),
			dependents:   make([]string, 0),
		}
	}

	for _, edge := range workflow.Edges {
		if source, ok := graph.nodes[edge.Source]; ok {
			source.dependents = append(source.dependents, edge.Target)
		}
		if target, ok := graph.nodes[edge.Target]; ok {
			target.dependencies = append(target.dependencies, edge.Source)
		}
	}

	return graph
}

func (gr *GraphRunnerV2) executeGraph(ctx context.Context, graph *executionGraph, workflow *Workflow) error {
	readyNodes := gr.findReadyNodes(graph)
	if len(readyNodes) == 0 {
		return fmt.Errorf("no executable nodes found")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(graph.nodes))
	semaphore := make(chan struct{}, 10)

	for len(readyNodes) > 0 {
		for _, nodeID := range readyNodes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			wg.Add(1)
			semaphore <- struct{}{}

			go func(nid string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				if err := gr.executeNode(ctx, graph, workflow, nid); err != nil {
					errChan <- err
				}
			}(nodeID)
		}

		wg.Wait()

		select {
		case err := <-errChan:
			return err
		default:
		}

		readyNodes = gr.findReadyNodes(graph)
	}

	return nil
}

func (gr *GraphRunnerV2) findReadyNodes(graph *executionGraph) []string {
	graph.mu.RLock()
	defer graph.mu.RUnlock()

	var ready []string

	for id, execNode := range graph.nodes {
		execNode.mu.RLock()
		if execNode.executed {
			execNode.mu.RUnlock()
			continue
		}

		allDepsExecuted := true
		for _, depID := range execNode.dependencies {
			if dep, ok := graph.nodes[depID]; ok {
				dep.mu.RLock()
				if !dep.executed {
					allDepsExecuted = false
				}
				dep.mu.RUnlock()
				if !allDepsExecuted {
					break
				}
			}
		}

		execNode.mu.RUnlock()

		if allDepsExecuted {
			ready = append(ready, id)
		}
	}

	return ready
}

func (gr *GraphRunnerV2) executeNode(ctx context.Context, graph *executionGraph, workflow *Workflow, nodeID string) error {
	execNode := graph.nodes[nodeID]
	node := execNode.node

	span, ctx := gr.tracer.StartSpan(ctx, "node.execute")
	defer span.End()

	span.SetAttributes(map[string]any{
		"node.id":   node.ID,
		"node.type": string(node.Type),
	})

	logger := gr.logger.WithNode(node.ID)
	logger.Info("node execution started")

	nodeStartTime := time.Now()
	defer func() {
		gr.metrics.Histogram("node.duration", nil, map[string]string{
			"node_type": string(node.Type),
		}).ObserveDuration(nodeStartTime)
	}()

	resolvedConfig, err := ResolveVariables(node.Config, gr.stateManager.ctx)
	if err != nil {
		span.SetStatus(observability.StatusCodeError, err.Error())
		return fmt.Errorf("variable resolution failed for %s: %w", node.ID, err)
	}

	input := map[string]any{
		"config": resolvedConfig,
	}

	var rawResult any

	cbKey := fmt.Sprintf("node:%s", node.Type)
	rawResult, err = RetryWithBackoff(ctx, gr.retryConfig, func(ctx context.Context) (any, error) {
		result, err := gr.circuitBreakers.Execute(ctx, cbKey, func() (any, error) {
			return gr.executeNodeWithWorkerPool(ctx, node, input)
		})
		if err == nil {
			rawResult = result
		}
		return result, err
	})

	if err != nil {
		span.SetStatus(observability.StatusCodeError, err.Error())
		logger.Error("node execution failed", slog.Any("error", err))
		gr.metrics.Counter("node.failures", map[string]string{
			"node_type": string(node.Type),
		}).Inc()
		return fmt.Errorf("node %s execution failed: %w", node.ID, err)
	}

	result := parseBlockResult(rawResult)

	gr.stateManager.SetResult(node.ID, result.Data)

	resBytes, _ := json.Marshal(result.Data)
	if err := gr.storage.SaveNodeResult(ctx, gr.stateManager.ctx.ExecutionID, node.ID, resBytes); err != nil {
		logger.Error("failed to save node result", slog.Any("error", err))
	}

	execNode.mu.Lock()
	execNode.executed = true
	execNode.result = result
	execNode.mu.Unlock()

	logger.Info("node execution completed", slog.String("port", result.Port))
	span.SetStatus(observability.StatusCodeOK, "")

	gr.metrics.Counter("node.executions", map[string]string{
		"node_type": string(node.Type),
		"status":    "success",
	}).Inc()

	return nil
}

func (gr *GraphRunnerV2) executeNodeWithWorkerPool(ctx context.Context, node *Node, input any) (any, error) {
	if node.Type == NodeTypeSetVar || node.Type == NodeTypeGetVar {
		return gr.executeVariableNode(node, input)
	}

	scriptPath := gr.getScriptPath(node.Type)
	if scriptPath == "" {
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}

	return gr.workerPool.Submit(ctx, scriptPath, input, 30*time.Second)
}

func (gr *GraphRunnerV2) executeVariableNode(node *Node, input any) (any, error) {
	inputMap, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input format")
	}

	config, ok := inputMap["config"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid config format")
	}

	if node.Type == NodeTypeSetVar {
		name, _ := config["name"].(string)
		value := config["value"]
		gr.stateManager.ctx.SetVar(name, value)
		return map[string]any{"data": map[string]any{"name": name, "value": value}}, nil
	}

	if node.Type == NodeTypeGetVar {
		name, _ := config["name"].(string)
		value := gr.stateManager.ctx.GetVar(name)
		return map[string]any{"data": map[string]any{"name": name, "value": value}}, nil
	}

	return nil, fmt.Errorf("unsupported variable node type")
}

func (gr *GraphRunnerV2) getScriptPath(nodeType NodeType) string {
	registry := NewBlockRegistry("pkg/blocks")
	manifest, ok := registry.Get(nodeType)
	if ok {
		return manifest.ScriptPath
	}
	return ""
}
