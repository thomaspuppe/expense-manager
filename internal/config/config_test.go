package config

import "testing"

func TestLoadFailsFastWhenSecretsMissing(t *testing.T) {
	t.Setenv("EXPENSE_PASSWORD", "")
	t.Setenv("EXPENSE_BEARER_TOKEN", "")
	t.Setenv("SESSION_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail when required secrets are missing, got nil error")
	}
}

func TestLoadAppliesDefaultsAndReadsSecrets(t *testing.T) {
	t.Setenv("EXPENSE_PASSWORD", "pw")
	t.Setenv("EXPENSE_BEARER_TOKEN", "tok")
	t.Setenv("SESSION_SECRET", "secret")
	t.Setenv("EXPENSE_DB", "")
	t.Setenv("EXPENSE_ADDR", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if c.DBPath != "expenses.db" {
		t.Errorf("DBPath = %q, want default expenses.db", c.DBPath)
	}
	if c.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want default 127.0.0.1:8080", c.Addr)
	}
	if c.Password != "pw" || c.BearerToken != "tok" || c.SessionSecret != "secret" {
		t.Errorf("secrets not read correctly: %+v", c)
	}
}
