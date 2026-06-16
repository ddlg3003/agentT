package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

const dateLayout = "2006-01-02"

// BI is the query_bi tool: it returns Cash Loan funnel metrics per partner for a
// date range, with day-over-day and month-over-month deltas computed from the
// underlying dataset so the model does not have to do the arithmetic itself.
type BI struct {
	fs mockFS
}

// NewBI builds the query_bi tool over the given mock base directory.
func NewBI(mockBase string) *BI { return &BI{fs: mockFS{base: mockBase}} }

var _ agent.Tool = (*BI)(nil)

type biCatalogEntry struct {
	Step string `json:"step"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Unit string `json:"unit"`
}

type biRecord struct {
	Date    string             `json:"date"`
	Partner string             `json:"partner"`
	Values  map[string]float64 `json:"values"`
}

type biFile struct {
	Catalog []biCatalogEntry `json:"metric_catalog"`
	Records []biRecord       `json:"records"`
}

type biInput struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Partners []string `json:"partners"`
}

// Definition advertises query_bi to the model.
func (b *BI) Definition() agent.ToolDef {
	return agent.ToolDef{
		Name: "query_bi",
		Description: "Return Cash Loan conversion-funnel metrics per partner for a date range, " +
			"with day-over-day (delta_day) and month-over-month (delta_mom) deltas. " +
			"Rates are fractions (0.21 = 21%); deltas for rates are absolute differences in fraction units. " +
			"Use this for whitelist, traffic, demand/pass-rule/filling/submission/approval/signing rates and E2E rate.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "from": {"type": "string", "description": "start date inclusive, YYYY-MM-DD"},
    "to": {"type": "string", "description": "end date inclusive, YYYY-MM-DD"},
    "partners": {"type": "array", "items": {"type": "string"}, "description": "optional partner filter, e.g. [\"SHB\",\"CAKE\"]; empty means all"}
  },
  "required": ["from", "to"]
}`),
	}
}

// Run filters the dataset to the requested window/partners and attaches deltas.
func (b *BI) Run(_ context.Context, input json.RawMessage) (string, error) {
	var in biInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	from, err := time.Parse(dateLayout, in.From)
	if err != nil {
		return "", fmt.Errorf("invalid from date %q: %w", in.From, err)
	}
	to, err := time.Parse(dateLayout, in.To)
	if err != nil {
		return "", fmt.Errorf("invalid to date %q: %w", in.To, err)
	}

	// daily_metrics.from_events.json is produced by scripts/aggregate_funnel.py
	// from the anonymized production event mart — the script→file→tool pipeline
	// is reproducible. (daily_metrics.synthetic.json holds the old hand-made demo
	// data, kept for reference but no longer read.)
	raw, err := b.fs.readFile("bi", "daily_metrics.from_events.json")
	if err != nil {
		return "", err
	}
	var file biFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return "", fmt.Errorf("parse bi data: %w", err)
	}

	// Index records by date+partner for O(1) delta lookups.
	index := make(map[string]biRecord, len(file.Records))
	for _, r := range file.Records {
		index[r.Date+"|"+r.Partner] = r
	}
	catalog := make(map[string]biCatalogEntry, len(file.Catalog))
	for _, c := range file.Catalog {
		catalog[c.Key] = c
	}
	wantPartner := toSet(in.Partners)

	type metricOut struct {
		Step     string  `json:"step"`
		Key      string  `json:"key"`
		Name     string  `json:"name"`
		Unit     string  `json:"unit"`
		Value    float64 `json:"value"`
		DeltaDay float64 `json:"delta_day"`
		DeltaMoM float64 `json:"delta_mom"`
		HasDay   bool    `json:"has_delta_day"`
		HasMoM   bool    `json:"has_delta_mom"`
	}
	type recordOut struct {
		Date    string      `json:"date"`
		Partner string      `json:"partner"`
		Metrics []metricOut `json:"metrics"`
	}

	var out []recordOut
	for _, r := range file.Records {
		d, err := time.Parse(dateLayout, r.Date)
		if err != nil {
			continue
		}
		if d.Before(from) || d.After(to) {
			continue
		}
		if len(wantPartner) > 0 && !wantPartner[r.Partner] {
			continue
		}

		prevDay, hasDay := index[d.AddDate(0, 0, -1).Format(dateLayout)+"|"+r.Partner]
		prevMoM, hasMoM := index[d.AddDate(0, -1, 0).Format(dateLayout)+"|"+r.Partner]

		ro := recordOut{Date: r.Date, Partner: r.Partner}
		for _, c := range file.Catalog {
			v, ok := r.Values[c.Key]
			if !ok {
				continue
			}
			m := metricOut{Step: c.Step, Key: c.Key, Name: c.Name, Unit: c.Unit, Value: v}
			if hasDay {
				if pv, ok := prevDay.Values[c.Key]; ok {
					m.DeltaDay = round(v - pv)
					m.HasDay = true
				}
			}
			if hasMoM {
				if pv, ok := prevMoM.Values[c.Key]; ok {
					m.DeltaMoM = round(v - pv)
					m.HasMoM = true
				}
			}
			ro.Metrics = append(ro.Metrics, m)
		}
		out = append(out, ro)
	}

	if len(out) == 0 {
		return fmt.Sprintf(`{"data":[],"note":"no BI data for range %s..%s"}`, in.From, in.To), nil
	}
	resp, err := json.Marshal(map[string]any{"data": out})
	if err != nil {
		return "", fmt.Errorf("marshal bi response: %w", err)
	}
	return string(resp), nil
}

func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// round trims floating-point noise to 4 decimals for stable, readable deltas.
func round(f float64) float64 {
	return float64(int64(f*10000+sign(f)*0.5)) / 10000
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
