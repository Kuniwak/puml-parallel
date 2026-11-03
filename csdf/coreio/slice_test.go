package coreio

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

func TestWriteSliceDrawsTheCutEdgesToPlaceholdersOnlyForThePicture(t *testing.T) {
	const chain = `@startuml
state "A" as a
state "B" as b
state "C" as c
c : n ; N
state "a_cut" as a_cut
a_cut : m ; N
[*] --> a_cut : $m' = 0
a_cut --> a : w
a --> b : x
b --> c : y ; ; $n' = 1
c --> c : z ; ; $n' = $n
c --> [*]
note as channels
  channel w
  channel x
  channel y
  channel z
end note
@enduml
`
	cases := map[string]struct {
		states  []string
		context int
		want    string
	}{
		"the initial state outside: the start edge drawn to its placeholder": {[]string{"c"}, 1, `@startuml
state "B" as b
state "C" as c
c : n ; N
b --> c : y ; ; $n' = 1
c --> c : z ; ; $n' = $n
c --> [*]
note as channels
  channel y
  channel z
end note
' CSDF-IGNORE-BEGIN
state "A …" as a_cut_cut
state "a_cut …" as a_cut_cut_cut
[*] --> a_cut_cut_cut : $m' = 0
a_cut_cut --> b : x
' CSDF-IGNORE-END
@enduml
`},
		"the initial state inside: the start edge as it is": {[]string{"a_cut"}, 0, `@startuml
state "a_cut" as a_cut
a_cut : m ; N
[*] --> a_cut : $m' = 0
' CSDF-IGNORE-BEGIN
state "A …" as a_cut_cut
a_cut --> a_cut_cut : w
' CSDF-IGNORE-END
@enduml
`},
		"nothing cut: no region": {[]string{"a"}, 9, chainSorted},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d, err := Read(chain)
			if err != nil {
				t.Fatal(err)
			}
			sliced, err := core.Slice(d, c.states, c.context)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := WriteSlice(sliced)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

const chainSorted = `@startuml
state "A" as a
state "a_cut" as a_cut
a_cut : m ; N
state "B" as b
state "C" as c
c : n ; N
[*] --> a_cut : $m' = 0
a --> b : x
a_cut --> a : w
b --> c : y ; ; $n' = 1
c --> c : z ; ; $n' = $n
c --> [*]
note as channels
  channel w
  channel x
  channel y
  channel z
end note
@enduml
`
