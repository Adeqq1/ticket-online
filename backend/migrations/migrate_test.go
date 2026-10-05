package migrations

import "testing"

func TestLoadMigrationsInVersionOrder(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 11 {
		t.Fatalf("migration count = %d, want 11", len(items))
	}
	for index, item := range items {
		if item.version != index+1 {
			t.Fatalf("migration[%d] version = %d, want %d", index, item.version, index+1)
		}
	}
	if len(statements(string(items[0].data))) < 6 {
		t.Fatal("initial migration was not split into executable statements")
	}
}

func TestStatementsIgnoresEmptyStatements(t *testing.T) {
	got := statements(" CREATE TABLE example (id INT); ; ")
	if len(got) != 1 || got[0] != "CREATE TABLE example (id INT)" {
		t.Fatalf("unexpected statements: %#v", got)
	}
}
