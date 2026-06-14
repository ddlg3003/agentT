package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBIRunDeltas(t *testing.T) {
	t.Parallel()
	bi := NewBI("../../../mock")

	out, err := bi.Run(context.Background(), json.RawMessage(`{"from":"2026-03-15","to":"2026-03-15","partners":["SHB"]}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var resp struct {
		Data []struct {
			Date    string `json:"date"`
			Partner string `json:"partner"`
			Metrics []struct {
				Key      string  `json:"key"`
				Value    float64 `json:"value"`
				DeltaDay float64 `json:"delta_day"`
				DeltaMoM float64 `json:"delta_mom"`
			} `json:"metrics"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(resp.Data))
	}
	rec := resp.Data[0]
	if rec.Partner != "SHB" || rec.Date != "2026-03-15" {
		t.Fatalf("unexpected record %s/%s", rec.Partner, rec.Date)
	}

	var e2e *float64
	var mom float64
	for _, m := range rec.Metrics {
		if m.Key == "e2e_rate" {
			v := m.Value
			e2e = &v
			mom = m.DeltaMoM
		}
	}
	if e2e == nil {
		t.Fatal("e2e_rate metric not found")
	}
	if *e2e != 0.037 {
		t.Errorf("e2e value = %v, want 0.037", *e2e)
	}
	// MoM = 0.037 (Mar-15) - 0.036 (Feb-15) = 0.001
	if mom != 0.001 {
		t.Errorf("e2e delta_mom = %v, want 0.001", mom)
	}
}

func TestBIRunEmptyRange(t *testing.T) {
	t.Parallel()
	bi := NewBI("../../../mock")
	out, err := bi.Run(context.Background(), json.RawMessage(`{"from":"2020-01-01","to":"2020-01-02"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("output not valid JSON: %s", out)
	}
	// Empty range should still return valid JSON with a note, not an error.
	if !strings.Contains(out, `"data":[]`) {
		t.Errorf("expected empty data note, got %s", out)
	}
}
