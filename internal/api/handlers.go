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

// expenseInput is the request body for create and update.
type expenseInput struct {
	AmountCents int64  `json:"amount_cents"`
	CategoryKey string `json:"category_key"`
	SpentOn     string `json:"spent_on"`
	Note        string `json:"note"`
}

// validate normalizes and checks an input. It defaults an empty date to today
// (local zone) and rejects a non-positive amount or an unknown category.
func (in *expenseInput) validate() (store.Expense, string) {
	if in.AmountCents <= 0 {
		return store.Expense{}, "amount_cents must be a positive integer (euro cents)"
	}
	if !store.IsValidCategory(in.CategoryKey) {
		return store.Expense{}, "unknown category_key"
	}
	spentOn := in.SpentOn
	if spentOn == "" {
		spentOn = todayLocal()
	} else if !validDate(spentOn) {
		return store.Expense{}, "spent_on must be YYYY-MM-DD"
	}
	return store.Expense{
		AmountCents: in.AmountCents,
		CategoryKey: in.CategoryKey,
		SpentOn:     spentOn,
		Note:        in.Note,
	}, ""
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
	e, msg := in.validate()
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	created, err := a.store.Create(e)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save expense")
		return
	}
	writeJSON(w, http.StatusCreated, created)
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
	e, msg := in.validate()
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
