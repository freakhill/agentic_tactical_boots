package promotion

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const MaxPlanBytes = 64 * 1024

type PlanFile struct {
	Kind string     `json:"kind"`
	Plan PlanIntent `json:"plan"`
}

func WritePlan(path string, plan PlanIntent) error {
	if path == "" {
		return ErrInvalidRequest
	}
	b, err := json.MarshalIndent(PlanFile{Kind: "safeslop-promotion-plan", Plan: plan}, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > MaxPlanBytes {
		return fmt.Errorf("promotion plan exceeds %d bytes", MaxPlanBytes)
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func ReadPlan(path string) (PlanIntent, error) {
	info, err := os.Stat(path)
	if err != nil {
		return PlanIntent{}, err
	}
	if !info.Mode().IsRegular() {
		return PlanIntent{}, ErrInvalidRequest
	}
	if info.Size() > MaxPlanBytes {
		return PlanIntent{}, fmt.Errorf("promotion plan exceeds %d bytes", MaxPlanBytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return PlanIntent{}, err
	}
	var f PlanFile
	if err := json.Unmarshal(b, &f); err != nil {
		return PlanIntent{}, err
	}
	if f.Kind != "safeslop-promotion-plan" || f.Plan.Version != PlanVersion || f.Plan.RendererVersion != RendererVersion || f.Plan.SchemaVersion != SchemaVersion {
		return PlanIntent{}, ErrInvalidRequest
	}
	return f.Plan, nil
}

func IsStaleSource(err error) bool { return errors.Is(err, ErrStaleSource) }
