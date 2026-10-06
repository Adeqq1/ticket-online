package migrations

import (
	"strings"
	"testing"
)

func TestLoadMigrationsInVersionOrder(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 14 {
		t.Fatalf("migration count = %d, want 14", len(items))
	}
	for index, item := range items {
		if item.version != index+1 {
			t.Fatalf("migration[%d] version = %d, want %d", index, item.version, index+1)
		}
	}
	if len(statements(string(items[0].data))) < 6 {
		t.Fatal("initial migration was not split into executable statements")
	}
	if !strings.Contains(string(items[12].data), "CREATE TABLE IF NOT EXISTS email_queue") {
		t.Fatal("email queue migration was not loaded")
	}
	if !strings.Contains(string(items[13].data), "CREATE TABLE IF NOT EXISTS recovery_tokens") {
		t.Fatal("ticket recovery migration was not loaded")
	}
}

func TestStatementsIgnoresEmptyStatements(t *testing.T) {
	got := statements(" CREATE TABLE example (id INT); ; ")
	if len(got) != 1 || got[0] != "CREATE TABLE example (id INT)" {
		t.Fatalf("unexpected statements: %#v", got)
	}
}
