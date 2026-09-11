package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"expensemanager/internal/store"
)

// localLoc is the owner's zone. "Today" and month boundaries are computed here,
// not in UTC, so a late-night entry lands in the correct day and month.
// tzdata is embedded in the binary (see main), so this resolves without system
// zoneinfo on the deploy host.
var localLoc = loadLocal()

func loadLocal() *time.Location {
	if loc, err := time.LoadLocation("Europe/Berlin"); err == nil {
		return loc
	}
	return time.Local
}

func todayLocal() string        { return time.Now().In(localLoc).Format("2006-01-02") }
func currentMonthLocal() string { return time.Now().In(localLoc).Format("2006-01") }

func validDate(s string) bool  { _, err := time.Parse("2006-01-02", s); return err == nil }
func validMonth(s string) bool { _, err := time.Parse("2006-01", s); return err == nil }

// expenseInput is the request body for create and update. The fields are
// pointers so a PATCH can distinguish "not mentioned" from "set to this" — the
// clients this API is built for (CSV import, MCP/AI) will patch a subset, and a
// field they leave out must keep its stored value rather than be blanked.
type expenseInput struct {
	AmountCents *int64  `json:"amount_cents"`
	CategoryKey *string `json:"category_key"`
	SpentOn     *string `json:"spent_on"`
	Note        *string `json:"note"`
}

// applyTo overlays the fields present in the body onto base — the zero Expense
// for a create, the stored expense for an update — and validates the result. An
// omitted date falls back to today (local zone) only when base has none to keep;
// a date that is present but unparseable is always an error, so clearing the
// field cannot quietly re-file an old expense under today.
func (in *expenseInput) applyTo(base store.Expense) (store.Expense, string) {
	e := base
	if in.AmountCents != nil {
		e.AmountCents = *in.AmountCents
	}
	if in.CategoryKey != nil {
		e.CategoryKey = *in.CategoryKey
	}
	if in.Note != nil {
		e.Note = *in.Note
	}
	if in.SpentOn != nil {
		if !validDate(*in.SpentOn) {
			return store.Expense{}, "spent_on must be YYYY-MM-DD"
		}
		e.SpentOn = *in.SpentOn
	} else if e.SpentOn == "" {
		e.SpentOn = todayLocal()
	}

	if e.AmountCents <= 0 {
		return store.Expense{}, "amount_cents must be a positive integer (euro cents)"
	}
	// Validate the category only when this request actually sets it: on create
	// base is the zero Expense, so there is always a category to check; on
	// update, a category_key the client didn't send should keep whatever was
	// already stored — including one later removed from the fixed set
	// (internal/store/categories.go) — rather than block every future edit to
	// that expense's amount/date/note just because its old category is gone.
	if (in.CategoryKey != nil || base.CategoryKey == "") && !store.IsValidCategory(e.CategoryKey) {
		return store.Expense{}, "unknown category_key"
	}
	return e, ""
}

func decodeInput(r *http.Request) (expenseInput, bool) {
	var in expenseInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return expenseInput{}, false
	}
	return in, true
}

func (a *API) createExpense(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	e, msg := in.applyTo(store.Expense{})
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	created, wasNew, err := a.store.CreateWithKey(e, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save expense")
		return
	}
	status := http.StatusCreated
	if !wasNew {
		status = http.StatusOK // idempotent replay: the key already produced this expense
	}
	writeJSON(w, status, created)
}

func (a *API) listExpenses(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonthLocal()
	} else if !validMonth(month) {
		writeErr(w, http.StatusBadRequest, "month must be YYYY-MM")
		return
	}
	list, err := a.store.ListByMonth(month)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list expenses")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonthLocal()
	} else if !validMonth(month) {
		writeErr(w, http.StatusBadRequest, "month must be YYYY-MM")
		return
	}
	sum, err := a.store.Summary(month)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not compute summary")
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (a *API) categories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.Categories)
}

func (a *API) updateExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	in, ok := decodeInput(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	current, err := a.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "expense not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "could not read expense")
		return
	}
	e, msg := in.applyTo(current)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	e.ID = id
	if err := a.store.Update(e); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "expense not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "could not update expense")
		return
	}
	updated, err := a.store.Get(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read updated expense")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "expense not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "could not delete expense")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
