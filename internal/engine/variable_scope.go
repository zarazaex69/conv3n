package engine

import (
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
	Value     interface{}
	Scope     VariableScope
	ExpiresAt *time.Time
}

type VariableStore struct {
	global    map[string]*ScopedVariable
	workflow  map[string]map[string]*ScopedVariable
	execution map[string]map[string]*ScopedVariable
	mu        sync.RWMutex
	stopChan  chan struct{}
}

func NewVariableStore() *VariableStore {
	store := &VariableStore{
		global:    make(map[string]*ScopedVariable),
		workflow:  make(map[string]map[string]*ScopedVariable),
		execution: make(map[string]map[string]*ScopedVariable),
		stopChan:  make(chan struct{}),
	}

	go store.cleanupExpired()

	return store
}

func (vs *VariableStore) Set(workflowID, executionID, name string, value interface{}, scope VariableScope, ttl *time.Duration) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var expiresAt *time.Time
	if ttl != nil {
		expiry := time.Now().Add(*ttl)
		expiresAt = &expiry
	}

	variable := &ScopedVariable{
		Value:     value,
		Scope:     scope,
		ExpiresAt: expiresAt,
	}

	switch scope {
	case ScopeGlobal:
		vs.global[name] = variable
	case ScopeWorkflow:
		if vs.workflow[workflowID] == nil {
			vs.workflow[workflowID] = make(map[string]*ScopedVariable)
		}
		vs.workflow[workflowID][name] = variable
	case ScopeExecution:
		if vs.execution[executionID] == nil {
			vs.execution[executionID] = make(map[string]*ScopedVariable)
		}
		vs.execution[executionID][name] = variable
	}
}

func (vs *VariableStore) Get(workflowID, executionID, name string) (interface{}, bool) {
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
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			vs.mu.Lock()
			vs.cleanupMap(vs.global)
			for _, wfVars := range vs.workflow {
				vs.cleanupMap(wfVars)
			}
			for _, execVars := range vs.execution {
				vs.cleanupMap(execVars)
			}
			vs.mu.Unlock()
		case <-vs.stopChan:
			return
		}
	}
}

func (vs *VariableStore) cleanupMap(m map[string]*ScopedVariable) {
	for name, v := range m {
		if vs.isExpired(v) {
			delete(m, name)
		}
	}
}

func (vs *VariableStore) Close() {
	close(vs.stopChan)
}
