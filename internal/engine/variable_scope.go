package engine

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

type VariableScope string

const (
	ScopeGlobal    VariableScope = "global"
	ScopeWorkflow  VariableScope = "workflow"
	ScopeExecution VariableScope = "execution"
)

type ScopedVariable struct {
	Value        any
	Scope        VariableScope
	ExpiresAt    *time.Time
	ExpectedType string
}

type expiryItem struct {
	key       string
	scope     VariableScope
	scopeKey  string
	expiresAt time.Time
	index     int
}

type expiryHeap []*expiryItem

func (h expiryHeap) Len() int           { return len(h) }
func (h expiryHeap) Less(i, j int) bool { return h[i].expiresAt.Before(h[j].expiresAt) }
func (h expiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *expiryHeap) Push(x any) {
	n := len(*h)
	item := x.(*expiryItem)
	item.index = n
	*h = append(*h, item)
}

func (h *expiryHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

type VariableStore struct {
	global             map[string]*ScopedVariable
	workflow           map[string]map[string]*ScopedVariable
	execution          map[string]map[string]*ScopedVariable
	mu                 sync.RWMutex
	stopChan           chan struct{}
	expiry             expiryHeap
	expiryMu           sync.Mutex
	disableShadowCheck bool
}

func NewVariableStore() *VariableStore {
	store := &VariableStore{
		global:    make(map[string]*ScopedVariable),
		workflow:  make(map[string]map[string]*ScopedVariable),
		execution: make(map[string]map[string]*ScopedVariable),
		stopChan:  make(chan struct{}),
		expiry:    make(expiryHeap, 0),
	}

	heap.Init(&store.expiry)
	go store.cleanupExpired()

	return store
}

func (vs *VariableStore) Set(workflowID, executionID, name string, value any, scope VariableScope, ttl *time.Duration) error {
	limiter := NewDataLimiter()
	if err := limiter.ValidateVariable(value); err != nil {
		return err
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	if !vs.disableShadowCheck {
		if err := vs.checkShadowing(workflowID, executionID, name, scope); err != nil {
			return err
		}
	}

	expectedType := inferType(value)

	var expiresAt *time.Time
	if ttl != nil {
		expiry := time.Now().Add(*ttl)
		expiresAt = &expiry
	}

	variable := &ScopedVariable{
		Value:        value,
		Scope:        scope,
		ExpiresAt:    expiresAt,
		ExpectedType: expectedType,
	}

	var scopeKey string
	switch scope {
	case ScopeGlobal:
		vs.global[name] = variable
		scopeKey = "global"
	case ScopeWorkflow:
		if vs.workflow[workflowID] == nil {
			vs.workflow[workflowID] = make(map[string]*ScopedVariable)
		}
		vs.workflow[workflowID][name] = variable
		scopeKey = workflowID
	case ScopeExecution:
		if vs.execution[executionID] == nil {
			vs.execution[executionID] = make(map[string]*ScopedVariable)
		}
		vs.execution[executionID][name] = variable
		scopeKey = executionID
	}

	if expiresAt != nil {
		vs.expiryMu.Lock()
		heap.Push(&vs.expiry, &expiryItem{
			key:       name,
			scope:     scope,
			scopeKey:  scopeKey,
			expiresAt: *expiresAt,
		})
		vs.expiryMu.Unlock()
	}

	return nil
}

func (vs *VariableStore) Get(workflowID, executionID, name string) (any, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if execVars, ok := vs.execution[executionID]; ok {
		if v, exists := execVars[name]; exists && !vs.isExpired(v) {
			return v.Value, true
		}
	}

	if wfVars, ok := vs.workflow[workflowID]; ok {
		if v, exists := wfVars[name]; exists && !vs.isExpired(v) {
			return v.Value, true
		}
	}

	if v, exists := vs.global[name]; exists && !vs.isExpired(v) {
		return v.Value, true
	}

	return nil, false
}

func (vs *VariableStore) GetWithTypeCheck(workflowID, executionID, name, expectedType string) (any, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var variable *ScopedVariable
	if execVars, ok := vs.execution[executionID]; ok {
		if v, exists := execVars[name]; exists && !vs.isExpired(v) {
			variable = v
		}
	}

	if variable == nil {
		if wfVars, ok := vs.workflow[workflowID]; ok {
			if v, exists := wfVars[name]; exists && !vs.isExpired(v) {
				variable = v
			}
		}
	}

	if variable == nil {
		if v, exists := vs.global[name]; exists && !vs.isExpired(v) {
			variable = v
		}
	}

	if variable == nil {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}

	if expectedType != "" && variable.ExpectedType != expectedType {
		return nil, fmt.Errorf("type mismatch: variable '%s' expected %s, got %s", name, expectedType, variable.ExpectedType)
	}

	return variable.Value, nil
}

func (vs *VariableStore) Delete(workflowID, executionID, name string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if execVars, ok := vs.execution[executionID]; ok {
		delete(execVars, name)
	}

	if wfVars, ok := vs.workflow[workflowID]; ok {
		delete(wfVars, name)
	}

	delete(vs.global, name)
}

func (vs *VariableStore) ClearExecution(executionID string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	delete(vs.execution, executionID)
}

func (vs *VariableStore) isExpired(v *ScopedVariable) bool {
	if v.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*v.ExpiresAt)
}

func (vs *VariableStore) cleanupExpired() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			vs.processExpiredVariables()
		case <-vs.stopChan:
			return
		}
	}
}

func (vs *VariableStore) processExpiredVariables() {
	now := time.Now()

	vs.expiryMu.Lock()
	defer vs.expiryMu.Unlock()

	for vs.expiry.Len() > 0 {
		item := vs.expiry[0]
		if item.expiresAt.After(now) {
			break
		}

		heap.Pop(&vs.expiry)

		vs.mu.Lock()
		switch item.scope {
		case ScopeGlobal:
			if v, exists := vs.global[item.key]; exists && vs.isExpired(v) {
				delete(vs.global, item.key)
			}
		case ScopeWorkflow:
			if wfVars, ok := vs.workflow[item.scopeKey]; ok {
				if v, exists := wfVars[item.key]; exists && vs.isExpired(v) {
					delete(wfVars, item.key)
				}
			}
		case ScopeExecution:
			if execVars, ok := vs.execution[item.scopeKey]; ok {
				if v, exists := execVars[item.key]; exists && vs.isExpired(v) {
					delete(execVars, item.key)
				}
			}
		}
		vs.mu.Unlock()
	}
}

func (vs *VariableStore) Close() {
	close(vs.stopChan)
}

func (vs *VariableStore) checkShadowing(workflowID, executionID, name string, scope VariableScope) error {
	switch scope {
	case ScopeExecution:
		if wfVars, ok := vs.workflow[workflowID]; ok {
			if _, exists := wfVars[name]; exists {
				return fmt.Errorf("shadowing error: variable '%s' already exists in workflow scope", name)
			}
		}
		if _, exists := vs.global[name]; exists {
			return fmt.Errorf("shadowing error: variable '%s' already exists in global scope", name)
		}
	case ScopeWorkflow:
		if _, exists := vs.global[name]; exists {
			return fmt.Errorf("shadowing error: variable '%s' already exists in global scope", name)
		}
	}
	return nil
}

func inferType(value any) string {
	if value == nil {
		return "nil"
	}
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "unknown"
	}
}
