package people

import (
	"testing"
)

func TestStore(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	var got []string
	s.Subscribe(func(p Person, what string) { got = append(got, p.Name+":"+what) })

	p, _ := s.Seen("Erick")
	if p.Role != Family || !p.Voice || !p.CanAct() {
		t.Fatal("a registered voice starts as family", p)
	}
	p, _ = s.Remember("Ana", "cumple el 3 de mayo")
	if p.Role != Guest || p.CanAct() {
		t.Fatal("someone BB-8 is told about starts as guest", p)
	}
	s.Remember("ana", "cumple el 3 de mayo") // same note, other case: no duplicate
	s.Remember("Ana", "no come gluten")
	if p, _ := s.Get("ANA"); len(p.Notes) != 2 {
		t.Fatal(p.Notes)
	}
	if n, _ := s.Forget("Ana", "GLUTEN"); n != 1 {
		t.Fatal("forget", n)
	}
	if _, err := s.Upsert("Erick", "admin"); err == nil {
		t.Fatal("bad role accepted")
	}
	s.Upsert("Erick", Owner)
	if s.Summary() != "Ana (guest), Erick (owner)" {
		t.Fatal(s.Summary())
	}
	for i := 0; i < MaxNotes+5; i++ {
		s.Remember("Erick", string(rune('a'+i)))
	}
	if p, _ := s.Get("Erick"); len(p.Notes) != MaxNotes || p.Notes[0].Text != "f" {
		t.Fatal("oldest notes go first", len(p.Notes), p.Notes[0])
	}

	// Persisted.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := s2.Get("erick"); !ok || p.Role != Owner {
		t.Fatal("reload", p)
	}
	if err := s2.Remove("Ana"); err != nil || len(s2.List()) != 1 {
		t.Fatal(err, s2.List())
	}
	if got[0] != "Erick:added" || got[1] != "Ana:added" && got[1] != "Ana:note" {
		t.Fatal(got)
	}
}
