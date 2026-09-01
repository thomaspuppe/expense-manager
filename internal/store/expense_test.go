package store

import (
	"errors"
	"testing"
)

func mustCreate(t *testing.T, s *Store, cents int64, cat, date string) Expense {
	t.Helper()
	e, err := s.Create(Expense{AmountCents: cents, CategoryKey: cat, SpentOn: date, Note: ""})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return e
}

func TestCreateThenGetRoundTrips(t *testing.T) {
	s := tempStore(t)
	created, err := s.Create(Expense{AmountCents: 1250, CategoryKey: "groceries", SpentOn: "2026-09-01", Note: "coffee"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AmountCents != 1250 || got.CategoryKey != "groceries" || got.SpentOn != "2026-09-01" || got.Note != "coffee" {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Get(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestListByMonthFiltersAndOrders(t *testing.T) {
	s := tempStore(t)
	mustCreate(t, s, 100, "groceries", "2026-08-31") // other month
	mustCreate(t, s, 200, "transport", "2026-09-05")
	mustCreate(t, s, 300, "groceries", "2026-09-20")

	list, err := s.ListByMonth("2026-09")
	if err != nil {
		t.Fatalf("ListByMonth: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rows for 2026-09, got %d", len(list))
	}
	// Newest date first.
	if list[0].SpentOn != "2026-09-20" || list[1].SpentOn != "2026-09-05" {
		t.Errorf("wrong order: %s then %s", list[0].SpentOn, list[1].SpentOn)
	}
}

// TestSummaryFiltersByMonth covers AE2: totals include only the selected month.
func TestSummaryFiltersByMonth(t *testing.T) {
	s := tempStore(t)
	mustCreate(t, s, 5000, "groceries", "2026-08-15") // excluded
	mustCreate(t, s, 1000, "groceries", "2026-09-02")
	mustCreate(t, s, 1500, "groceries", "2026-09-10")
	mustCreate(t, s, 2000, "transport", "2026-09-11")

	sum, err := s.Summary("2026-09")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalCents != 4500 {
		t.Errorf("total = %d, want 4500 (August excluded)", sum.TotalCents)
	}
	byCat := map[string]int64{}
	for _, ct := range sum.ByCategory {
		byCat[ct.CategoryKey] = ct.TotalCents
	}
	if byCat["groceries"] != 2500 {
		t.Errorf("groceries = %d, want 2500", byCat["groceries"])
	}
	if byCat["transport"] != 2000 {
		t.Errorf("transport = %d, want 2000", byCat["transport"])
	}
}

func TestSummaryEmptyMonth(t *testing.T) {
	s := tempStore(t)
	sum, err := s.Summary("2026-01")
	if err != nil {
		t.Fatalf("Summary empty: %v", err)
	}
	if sum.TotalCents != 0 {
		t.Errorf("total = %d, want 0", sum.TotalCents)
	}
	if sum.ByCategory == nil {
		t.Error("ByCategory should be non-nil empty slice, not nil")
	}
	if len(sum.ByCategory) != 0 {
		t.Errorf("expected empty breakdown, got %d entries", len(sum.ByCategory))
	}
}

func TestUpdateChangesFields(t *testing.T) {
	s := tempStore(t)
	e := mustCreate(t, s, 1000, "groceries", "2026-09-05")

	e.AmountCents = 1750
	e.CategoryKey = "eating_out"
	e.SpentOn = "2026-09-06"
	e.Note = "lunch"
	if err := s.Update(e); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(e.ID)
	if got.AmountCents != 1750 || got.CategoryKey != "eating_out" || got.SpentOn != "2026-09-06" || got.Note != "lunch" {
		t.Errorf("update not reflected: %+v", got)
	}
}

func TestUpdateMissingReturnsNotFound(t *testing.T) {
	s := tempStore(t)
	if err := s.Update(Expense{ID: 42, AmountCents: 1, CategoryKey: "other", SpentOn: "2026-09-01"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteRemoves(t *testing.T) {
	s := tempStore(t)
	e := mustCreate(t, s, 500, "other", "2026-09-01")

	if err := s.Delete(e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expense still present after delete: %v", err)
	}
	if err := s.Delete(e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: got %v, want ErrNotFound", err)
	}
}

func TestEditedDateMovesMonth(t *testing.T) {
	s := tempStore(t)
	e := mustCreate(t, s, 900, "home", "2026-09-30")

	e.SpentOn = "2026-10-01"
	if err := s.Update(e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := s.ListByMonth("2026-09"); len(got) != 0 {
		t.Errorf("expected 0 in September after date move, got %d", len(got))
	}
	if got, _ := s.ListByMonth("2026-10"); len(got) != 1 {
		t.Errorf("expected 1 in October after date move, got %d", len(got))
	}
}
