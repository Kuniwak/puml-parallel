package animation

import (
	"strings"
	"testing"
)

type fakeSuggester struct {
	NoSuggester
	initial []Suggestion
	next    map[int][]Suggestion
	asked   []int
}

func (f *fakeSuggester) Initial() ([]Suggestion, error) {
	f.asked = append(f.asked, -1)
	return f.initial, nil
}
func (f *fakeSuggester) Next(from State, idx int) ([]Suggestion, error) {
	f.asked = append(f.asked, idx)
	return f.next[idx], nil
}

func TestSuggestAsksForTheInitialStateThenForTheSelectedTransition(t *testing.T) {
	fake := &fakeSuggester{initial: []Suggestion{{Values: []byte(`[2]`)}}, next: map[int][]Suggestion{1: {{Values: []byte(`[]`)}}}}
	sess, err := NewSession(mustRead(t, counterSource), fake)
	if err != nil {
		t.Fatalf("Setup: NewSession() error = %v", err)
	}

	got, err := sess.Suggest()
	if err != nil || len(got) != 1 || string(got[0].Values) != "[2]" {
		t.Fatalf("Suggest() = %v, %v; want the initial suggestion", got, err)
	}
	if _, err := sess.EnterValues("[0]"); err != nil {
		t.Fatal(err)
	}
	if got, _ := sess.Suggest(); got != nil {
		t.Errorf("Suggest() in command mode = %v, want nil", got)
	}
	if err := sess.Select(1); err != nil { // stop
		t.Fatal(err)
	}
	got, err = sess.Suggest()
	if err != nil || len(got) != 1 {
		t.Fatalf("Suggest() after select = %v, %v", got, err)
	}
	if len(fake.asked) != 2 || fake.asked[1] != 1 {
		t.Errorf("asked = %v, want the initial state then edge 1", fake.asked)
	}
}

func TestPickSuggestion(t *testing.T) {
	ss := []Suggestion{{Values: []byte(`[1]`)}, {Values: []byte(`[2]`)}}
	if v, ok, err := PickSuggestion("#1", ss); !ok || err != nil || v != "[2]" {
		t.Errorf("#1 -> %q %t %v", v, ok, err)
	}
	if _, ok, _ := PickSuggestion("[3]", ss); ok {
		t.Error("a plain JSON array is not a pick")
	}
	if _, ok, err := PickSuggestion("#7", ss); !ok || err == nil {
		t.Error("#7 should be a pick that fails")
	}
	var b strings.Builder
	RenderSuggestions(&b, ss)
	if !strings.Contains(b.String(), "[#0] [1]") || !strings.Contains(b.String(), "[#1] [2]") {
		t.Errorf("RenderSuggestions = %q", b.String())
	}
}
