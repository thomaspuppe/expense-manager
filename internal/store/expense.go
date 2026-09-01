package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound is returned when an expense id does not exist.
var ErrNotFound = errors.New("expense not found")

// Expense is one logged outgoing amount. Money is stored as integer euro cents
// (KTD2). SpentOn is a YYYY-MM-DD date in the owner's local zone.
type Expense struct {
	ID          int64  `json:"id"`
	AmountCents int64  `json:"amount_cents"`
	CategoryKey string `json:"category_key"`
	SpentOn     string `json:"spent_on"`
	Note        string `json:"note"`
	CreatedAt   string `json:"created_at"`
}

// CategoryTotal is one category's spend within a month.
type CategoryTotal struct {
	CategoryKey string `json:"category_key"`
	TotalCents  int64  `json:"total_cents"`
}

// Summary is a month's total plus its per-category breakdown.
type Summary struct {
	Month      string          `json:"month"`
	TotalCents int64           `json:"total_cents"`
	ByCategory []CategoryTotal `json:"by_category"`
}

// Create inserts e and returns it with ID and CreatedAt populated.
func (s *Store) Create(e Expense) (Expense, error) {
	created, _, err := s.CreateWithKey(e, "")
	return created, err
}

// CreateWithKey inserts e, optionally guarded by an idempotency key. When key is
// non-empty and an expense with that key already exists, the existing expense is
// returned with created=false — so a retried capture (e.g. after an offline
// error) cannot create a duplicate. An empty key stores NULL and always inserts.
func (s *Store) CreateWithKey(e Expense, key string) (Expense, bool, error) {
	if key != "" {
		existing, err := s.getByKey(key)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Expense{}, false, err
		}
	}

	e.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	var keyArg any
	if key != "" {
		keyArg = key
	}
	res, err := s.db.Exec(
		`INSERT INTO expenses (amount_cents, category_key, spent_on, note, created_at, idempotency_key)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.AmountCents, e.CategoryKey, e.SpentOn, e.Note, e.CreatedAt, keyArg,
	)
	if err != nil {
		return Expense{}, false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Expense{}, false, err
	}
	e.ID = id
	return e, true, nil
}

func (s *Store) getByKey(key string) (Expense, error) {
	var e Expense
	err := s.db.QueryRow(
		`SELECT id, amount_cents, category_key, spent_on, note, created_at
		 FROM expenses WHERE idempotency_key = ?`, key,
	).Scan(&e.ID, &e.AmountCents, &e.CategoryKey, &e.SpentOn, &e.Note, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, ErrNotFound
	}
	if err != nil {
		return Expense{}, err
	}
	return e, nil
}

// Get returns the expense with the given id, or ErrNotFound.
func (s *Store) Get(id int64) (Expense, error) {
	var e Expense
	err := s.db.QueryRow(
		`SELECT id, amount_cents, category_key, spent_on, note, created_at
		 FROM expenses WHERE id = ?`, id,
	).Scan(&e.ID, &e.AmountCents, &e.CategoryKey, &e.SpentOn, &e.Note, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, ErrNotFound
	}
	if err != nil {
		return Expense{}, err
	}
	return e, nil
}

// ListByMonth returns all expenses in the given month (YYYY-MM), most recent
// first (by date, then by insertion order).
func (s *Store) ListByMonth(month string) ([]Expense, error) {
	rows, err := s.db.Query(
		`SELECT id, amount_cents, category_key, spent_on, note, created_at
		 FROM expenses
		 WHERE substr(spent_on, 1, 7) = ?
		 ORDER BY spent_on DESC, id DESC`, month,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expenses := []Expense{}
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.AmountCents, &e.CategoryKey, &e.SpentOn, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

// Update writes amount, category, date, and note for the expense with e.ID.
// Returns ErrNotFound when no such expense exists.
func (s *Store) Update(e Expense) error {
	res, err := s.db.Exec(
		`UPDATE expenses
		 SET amount_cents = ?, category_key = ?, spent_on = ?, note = ?
		 WHERE id = ?`,
		e.AmountCents, e.CategoryKey, e.SpentOn, e.Note, e.ID,
	)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

// Delete removes the expense with the given id. Returns ErrNotFound when absent.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM expenses WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

// Summary returns the month's total and per-category breakdown. An empty month
// yields a zero total and an empty (non-nil) breakdown, not an error.
func (s *Store) Summary(month string) (Summary, error) {
	sum := Summary{Month: month, ByCategory: []CategoryTotal{}}

	rows, err := s.db.Query(
		`SELECT category_key, SUM(amount_cents) AS total
		 FROM expenses
		 WHERE substr(spent_on, 1, 7) = ?
		 GROUP BY category_key
		 ORDER BY total DESC`, month,
	)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var ct CategoryTotal
		if err := rows.Scan(&ct.CategoryKey, &ct.TotalCents); err != nil {
			return Summary{}, err
		}
		sum.ByCategory = append(sum.ByCategory, ct)
		sum.TotalCents += ct.TotalCents
	}
	return sum, rows.Err()
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
