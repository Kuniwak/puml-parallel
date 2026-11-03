package coreio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWritePromotionCanonical(t *testing.T) {
	p, err := ReadPromotion(`@startuml G
' a comment for the picture
state "稼働中" as running {
  running : n ; Nat
  running : reviews ; ReviewId ⇸ Review
  running : members ; MemberId ⇸ Member
  state "reviews : (r : ReviewId) ⇸ Review" as rR <<promote>> {
    !include local/REVIEW.puml
  }
  state "members : (m : MemberId) ⇸ Member" as rM <<promote>>
}
state "縮退中" as degraded {
  degraded : reviews ; ReviewId ⇸ Review
  degraded : members ; MemberId ⇸ Member
  state "members : (m : MemberId) ⇸ Member" as dM <<promote>> {
    !include "local/MEMBER.puml"
  }
}
[*] --> running : $n' = 0
running --> degraded : DEGRADE
degraded --> running : RECOVER
note as z
  assume fair
end note
note as s2
  sync REVIEW_END : reviews(r).END, members(m)
end note
note as s1
  sync BEGIN : reviews(r), members(m).BEGIN_REVIEW ; $n > 0
end note
note bottom of rR
  constrain REVIEW?r ; $r は x
end note
note as c
  constrain BEGIN?r?m ; $m は y
  prose under the directive
end note
@enduml
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := WritePromotion(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `@startuml G
state "縮退中" as degraded {
  degraded : members ; MemberId ⇸ Member
  degraded : reviews ; ReviewId ⇸ Review
  state "members : (m : MemberId) ⇸ Member" as degraded_members <<promote>> {
    !include local/MEMBER.puml
  }
}
state "稼働中" as running {
  running : n ; Nat
  running : members ; MemberId ⇸ Member
  running : reviews ; ReviewId ⇸ Review
  state "members : (m : MemberId) ⇸ Member" as running_members <<promote>>
  state "reviews : (r : ReviewId) ⇸ Review" as running_reviews <<promote>> {
    !include local/REVIEW.puml
  }
}
[*] --> running : $n' = 0
degraded --> running : RECOVER
running --> degraded : DEGRADE
note as sync_BEGIN
  sync BEGIN : reviews(r), members(m).BEGIN_REVIEW ; $n > 0
end note
note as sync_REVIEW_END
  sync REVIEW_END : reviews(r).END, members(m)
end note
note as constrain1
  constrain BEGIN?r?m ; $m は y
end note
note as constrain2
  constrain REVIEW?r ; $r は x
end note
note as z
  assume fair
end note
@enduml
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("WritePromotion (-want +got):\n%s", diff)
	}
}

func TestWritePromotionExpandsAsTheSourceAndIsIdempotent(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "promote")
	paths, err := filepath.Glob(filepath.Join(dir, "*.puml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, ".expanded.puml") {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sorted := writePromotion(t, string(raw))
			if again := writePromotion(t, sorted); again != sorted {
				t.Errorf("not idempotent (-once +twice):\n%s", cmp.Diff(sorted, again))
			}
			// The order of the families is the order of the conjuncts and of
			// the maps of the expansion, which means nothing.
			if want, got := unordered(expand(t, string(raw), dir)), unordered(expand(t, sorted, dir)); want != got {
				t.Errorf("expands otherwise (-source +sorted):\n%s", cmp.Diff(want, got))
			}
		})
	}
}

func writePromotion(t *testing.T, source string) string {
	t.Helper()
	p, err := ReadPromotion(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, err := WritePromotion(p)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func expand(t *testing.T, source, dir string) string {
	t.Helper()
	p, err := ReadPromotion(source, FileIncluder(dir))
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := Expand(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, err := Write(d)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// unordered is the lines of a written diagram sorted, the conjuncts of each
// clause on an edge sorted, at every depth.
func unordered(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		head, label, ok := strings.Cut(l, " : ")
		if !ok || !strings.Contains(head, "-->") {
			continue
		}
		parts := strings.Split(label, " ; ")
		for j := range parts {
			parts[j] = conjunctsSorted(parts[j])
		}
		lines[i] = head + " : " + strings.Join(parts, " ; ")
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// conjunctsSorted is the clause text c with its conjuncts sorted, and those
// of the body of each @exists.
func conjunctsSorted(c string) string {
	var conjuncts []string
	depth, from := 0, 0
	for i, r := range c {
		switch r {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
		}
		if depth == 0 && strings.HasPrefix(c[i:], " ∧ ") {
			conjuncts = append(conjuncts, c[from:i])
			from = i + len(" ∧ ")
		}
	}
	conjuncts = append(conjuncts, c[from:])
	for i, f := range conjuncts {
		if head, body, ok := strings.Cut(f, ") ("); ok && strings.HasPrefix(f, "@exists(") && strings.HasSuffix(body, ")") {
			conjuncts[i] = head + ") (" + conjunctsSorted(strings.TrimSuffix(body, ")")) + ")"
		}
	}
	slices.Sort(conjuncts)
	return strings.Join(conjuncts, " ∧ ")
}
