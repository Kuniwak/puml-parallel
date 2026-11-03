package coreio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWritePromotionCanonical(t *testing.T) {
	// Setup
	source := `@startuml G
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
`
	p, err := ReadPromotionAsWritten(source)
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := WritePromotion(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := `@startuml G
state "縮退中" as degraded {
  degraded : reviews ; ReviewId ⇸ Review
  degraded : members ; MemberId ⇸ Member
  state "members : (m : MemberId) ⇸ Member" as dM <<promote>> {
    !include local/MEMBER.puml
  }
}
state "稼働中" as running {
  running : n ; Nat
  running : reviews ; ReviewId ⇸ Review
  running : members ; MemberId ⇸ Member
  state "members : (m : MemberId) ⇸ Member" as rM <<promote>>
  state "reviews : (r : ReviewId) ⇸ Review" as rR <<promote>> {
    !include local/REVIEW.puml
  }
}
[*] --> running : $n' = 0
degraded --> running : RECOVER
running --> degraded : DEGRADE
note as s1
  sync BEGIN : reviews(r), members(m).BEGIN_REVIEW ; $n > 0
end note
note as s2
  sync REVIEW_END : reviews(r).END, members(m)
end note
note as c
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
			if want, got := expand(t, string(raw), dir), expand(t, sorted, dir); want != got {
				t.Errorf("expands otherwise (-source +sorted):\n%s", cmp.Diff(want, got))
			}
		})
	}
}

func writePromotion(t *testing.T, source string) string {
	t.Helper()
	p, err := ReadPromotionAsWritten(source)
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

func TestWritePromotionRefusesAGeneratedIDThatClashes(t *testing.T) {
	// Setup
	p, err := ReadPromotionAsWritten(`@startuml
state "稼働中" as constrain1 {
  state "receipts : (r : ReceiptId) ⇸ Receipt" as rR <<promote>> {
    !include local/RECEIPT.puml
  }
}
[*] --> constrain1
note bottom of rR
  constrain RECEIVE?r ; $r は x
end note
@enduml
`)
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = WritePromotion(p)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "constrain1") {
		t.Errorf("want an error naming constrain1, got %v", err)
	}
}
