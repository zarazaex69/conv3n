package engine

import (
	"encoding/json"
	"fmt"
)

const (
	MaxNodeResultSize = 10 * 1024 * 1024
	MaxVariableSize   = 1 * 1024 * 1024
	MaxConfigSize     = 100 * 1024
)

type DataLimiter struct {
	maxNodeResultSize int64
	maxVariableSize   int64
	maxConfigSize     int64
}

func NewDataLimiter() *DataLimiter {
	return &DataLimiter{
		maxNodeResultSize: MaxNodeResultSize,
		maxVariableSize:   MaxVariableSize,
		maxConfigSize:     MaxConfigSize,
	}
}

func (dl *DataLimiter) ValidateNodeResult(data any) error {
	size, err := estimateSize(data)
	if err != nil {
		return fmt.Errorf("failed to estimate result size: %w", err)
	}

	if size > dl.maxNodeResultSize {
		return fmt.Errorf("node result size %d exceeds limit %d", size, dl.maxNodeResultSize)
	}

	return nil
}

func (dl *DataLimiter) ValidateVariable(data any) error {
	size, err := estimateSize(data)
	if err != nil {
		return fmt.Errorf("failed to estimate variable size: %w", err)
	}

	if size > dl.maxVariableSize {
		return fmt.Errorf("variable size %d exceeds limit %d", size, dl.maxVariableSize)
	}

	return nil
}

func (dl *DataLimiter) ValidateConfig(config map[string]any) error {
	size, err := estimateSize(config)
	if err != nil {
		return fmt.Errorf("failed to estimate config size: %w", err)
	}

	if size > dl.maxConfigSize {
		return fmt.Errorf("config size %d exceeds limit %d", size, dl.maxConfigSize)
	}

	return nil
}

func estimateSize(data any) (int64, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	return int64(len(bytes)), nil
}
