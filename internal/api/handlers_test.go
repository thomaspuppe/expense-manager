package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"expensemanager/internal/store"
)

func testAPI(t *testing.T) (*http.ServeMux, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	mux := http.NewServeMux()
	New(st).Register(mux)
	return mux, st
}

func do(t *testing.T, mux http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// TestCreateReturnsSavedExpenseDatedToday covers AE1.
func TestCreateReturnsSavedExpenseDatedToday(t *testing.T) {
	mux, _ := testAPI(t)
	rec := do(t, mux, "POST", "/api/expenses", map[string]any{
		"amount_cents": 1250,
		"category_key": "groceries",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var got store.Expense
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ID == 0 || got.AmountCents != 1250 || got.CategoryKey != "groceries" {
		t.Errorf("unexpected expense: %+v", got)
	}
	if got.SpentOn != todayLocal() {
		t.Errorf("spent_on = %q, want today %q", got.SpentOn, todayLocal())
	}
}

func TestCreateRejectsMissingAmount(t *testing.T) {
	mux, st := testAPI(t)
	rec := do(t, mux, "POST", "/api/expenses", map[string]any{"category_key": "groceries"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if list, _ := st.ListByMonth(currentMonthLocal()); len(list) != 0 {
		t.Errorf("no row should have been created, got %d", len(list))
	}
}

func TestCreateRejectsUnknownCategory(t *testing.T) {
	mux, _ := testAPI(t)
	rec := do(t, mux, "POST", "/api/expenses", map[string]any{
		"amount_cents": 500,
		"category_key": "not_real",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// TestSummaryLimitsToMonth covers AE2.
func TestSummaryLimitsToMonth(t *testing.T) {
	mux, st := testAPI(t)
	st.Create(store.Expense{AmountCents: 5000, CategoryKey: "groceries", SpentOn: "2026-08-01"})
	st.Create(store.Expense{AmountCents: 1000, CategoryKey: "groceries", SpentOn: "2026-09-03"})
	st.Create(store.Expense{AmountCents: 2000, CategoryKey: "transport", SpentOn: "2026-09-04"})

	rec := do(t, mux, "GET", "/api/summary?month=2026-09", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var sum store.Summary
	json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.TotalCents != 3000 {
		t.Errorf("total = %d, want 3000 (August excluded)", sum.TotalCents)
	}
}

func TestListDefaultsToCurrentMonth(t *testing.T) {
	mux, st := testAPI(t)
	st.Create(store.Expense{AmountCents: 700, CategoryKey: "home", SpentOn: todayLocal()})

	rec := do(t, mux, "GET", "/api/expenses", nil) // no month param
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var list []store.Expense
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("expected 1 expense for current month, got %d", len(list))
	}
}

func TestPatchThenDelete(t *testing.T) {
	mux, st := testAPI(t)
	e, _ := st.Create(store.Expense{AmountCents: 1000, CategoryKey: "groceries", SpentOn: "2026-09-05"})

	rec := do(t, mux, "PATCH", "/api/expenses/"+itoa(e.ID), map[string]any{
		"amount_cents": 1750, "category_key": "eating_out", "spent_on": "2026-09-06", "note": "lunch",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var updated store.Expense
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.AmountCents != 1750 || updated.CategoryKey != "eating_out" {
		t.Errorf("patch not applied: %+v", updated)
	}

	rec = do(t, mux, "DELETE", "/api/expenses/"+itoa(e.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = do(t, mux, "DELETE", "/api/expenses/"+itoa(e.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", rec.Code)
	}
}

func TestCategoriesReturnsTen(t *testing.T) {
	mux, _ := testAPI(t)
	rec := do(t, mux, "GET", "/api/categories", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var cats []store.Category
	json.Unmarshal(rec.Body.Bytes(), &cats)
	if len(cats) != 10 {
		t.Errorf("expected 10 categories, got %d", len(cats))
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
