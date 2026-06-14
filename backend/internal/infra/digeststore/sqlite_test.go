package digeststore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vngcloud/agentt/internal/domain/digest"
)

func TestSQLiteRoundtrip(t *testing.T) {
	t.Parallel()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Not found before save.
	if _, err := store.Get(ctx, "2026-03-15"); !errors.Is(err, digest.ErrNotFound) {
		t.Fatalf("Get before save err = %v, want ErrNotFound", err)
	}

	d := digest.DailyDigest{
		Date:        "2026-03-15",
		GeneratedAt: time.Date(2026, 3, 16, 6, 0, 0, 0, time.UTC),
		Metrics:     []digest.Metric{{Partner: "SHB", Step: "s20s120", Value: 0.037}},
		Reasoning:   "E2E flat MoM",
	}
	if err := store.Save(ctx, d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "2026-03-15")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Reasoning != "E2E flat MoM" || len(got.Metrics) != 1 || got.Metrics[0].Value != 0.037 {
		t.Errorf("roundtrip mismatch: %+v", got)
	}

	// Upsert: saving again with same date overwrites, not duplicates.
	d.Reasoning = "updated"
	if err := store.Save(ctx, d); err != nil {
		t.Fatalf("Save (upsert): %v", err)
	}

	// Another month, for ListMonth/ListDates checks.
	if err := store.Save(ctx, digest.DailyDigest{Date: "2026-02-15", GeneratedAt: time.Now()}); err != nil {
		t.Fatalf("Save Feb: %v", err)
	}

	dates, err := store.ListDates(ctx)
	if err != nil {
		t.Fatalf("ListDates: %v", err)
	}
	if len(dates) != 2 || dates[0] != "2026-03-15" {
		t.Errorf("ListDates = %v, want newest-first [2026-03-15 2026-02-15]", dates)
	}

	march, err := store.ListMonth(ctx, "2026-03")
	if err != nil {
		t.Fatalf("ListMonth: %v", err)
	}
	if len(march) != 1 || march[0].Reasoning != "updated" {
		t.Errorf("ListMonth(2026-03) = %+v, want 1 updated digest", march)
	}
}
