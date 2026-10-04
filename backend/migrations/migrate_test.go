package migrations

import "testing"

func TestLoadMigrationsInVersionOrder(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 8 || items[0].version != 1 || items[1].version != 2 || items[2].version != 3 || items[3].version != 4 || items[4].version != 5 || items[5].version != 6 || items[6].version != 7 || items[7].version != 8 {
		t.Fatalf("unexpected migrations: %+v", items)
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
