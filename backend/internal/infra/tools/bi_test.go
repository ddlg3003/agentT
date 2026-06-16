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

	// Real data (daily_metrics.from_events.json) has adjacent days 04-11 and 04-12
	// for SHB, so day-over-day deltas are exercised; there is no month-earlier pair.
	out, err := bi.Run(context.Background(), json.RawMessage(`{"from":"2026-04-12","to":"2026-04-12","partners":["SHB"]}`))
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
				HasDay   bool    `json:"has_delta_day"`
				HasMoM   bool    `json:"has_delta_mom"`
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
	if rec.Partner != "SHB" || rec.Date != "2026-04-12" {
		t.Fatalf("unexpected record %s/%s", rec.Partner, rec.Date)
	}

	var found bool
	for _, m := range rec.Metrics {
		if m.Key != "e2e_rate" {
			continue
		}
		found = true
		if m.Value != 0.0303 {
			t.Errorf("e2e value = %v, want 0.0303", m.Value)
		}
		// delta_day = 0.0303 (04-12) - 0.0519 (04-11) = -0.0216
		if !m.HasDay {
			t.Errorf("expected day-over-day delta to be present")
		}
		if m.DeltaDay != -0.0216 {
			t.Errorf("e2e delta_day = %v, want -0.0216", m.DeltaDay)
		}
		// No 2026-03-12 record exists, so MoM must be absent.
		if m.HasMoM {
			t.Errorf("expected no month-over-month delta, got %v", m.DeltaMoM)
		}
	}
	if !found {
		t.Fatal("e2e_rate metric not found")
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
