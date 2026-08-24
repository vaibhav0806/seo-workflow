package gsc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const performanceHistoryLimit = 8

type PerformanceState struct {
	Snapshots []PerformanceSnapshot `json:"snapshots"`
}

func LoadPerformanceState(path string) (PerformanceState, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return PerformanceState{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PerformanceState{}, nil
		}
		return PerformanceState{}, fmt.Errorf("read performance state: %w", err)
	}
	var state PerformanceState
	if err := json.Unmarshal(raw, &state); err != nil {
		return PerformanceState{}, fmt.Errorf("decode performance state: %w", err)
	}
	return state, nil
}

func SavePerformanceState(path string, state PerformanceState) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode performance state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create performance state directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".performance-*.json")
	if err != nil {
		return fmt.Errorf("create performance state temp file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(append(raw, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("write performance state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close performance state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace performance state: %w", err)
	}
	return nil
}

func AppendPerformanceSnapshot(state PerformanceState, snapshot PerformanceSnapshot) PerformanceState {
	state.Snapshots = append(state.Snapshots, snapshot)
	if len(state.Snapshots) > performanceHistoryLimit {
		state.Snapshots = append([]PerformanceSnapshot(nil), state.Snapshots[len(state.Snapshots)-performanceHistoryLimit:]...)
	}
	return state
}

func LatestPerformanceSnapshots(state PerformanceState) (PerformanceSnapshot, PerformanceSnapshot) {
	if len(state.Snapshots) == 0 {
		return PerformanceSnapshot{}, PerformanceSnapshot{}
	}
	current := state.Snapshots[len(state.Snapshots)-1]
	if len(state.Snapshots) == 1 {
		return current, PerformanceSnapshot{}
	}
	return current, state.Snapshots[len(state.Snapshots)-2]
}
