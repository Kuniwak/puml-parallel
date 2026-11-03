package csdfsortcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOK(t *testing.T) {
	type testCase struct {
		Args  []string
		Stdin string
		Want  string
	}

	testCases := map[string]testCase{
		"a file argument (a hand-written diagram in authoring order)": {
			Args: []string{filepath.Join("testdata", "unsorted.puml")},
			Want: `@startuml
state "First" as s0
state "Second" as s1
[*] --> s1
s0 --> s1 : a
s1 --> s0 : a ; g
s1 --> s0 : b
@enduml
`,
		},
		"standard input (equivalent to a file argument)": {
			Args: []string{},
			Stdin: `@startuml
state "s1" as s1
state "s0" as s0
[*] --> s0
s0 --> s1 : a
@enduml
`,
			Want: `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`,
		},
		"a diagram whose clauses use the builtins (clauses kept as written)": {
			Args: []string{filepath.Join("testdata", "builtins.puml")},
			Want: `@startuml
state "Door" as door
door : doors ; doors
door : loc ; loc
state "Shop" as shop
shop : doors ; doors
shop : loc ; loc
[*] --> shop : @empty(doors') ∧ $loc' = 0
door --> shop : close ; @ne(loc, 0) ; @eq(doors', doors) ∧ @eq(loc', loc)
shop --> shop : add?k ; @notin(k, doors) ; @update(doors', doors, k, k) ∧ @eq(loc', loc)
shop --> door : open?k ; @in(k, doors) ∧ @at(doors[k], shut) ; @frame(doors', doors, k) ∧ @at(doors'[k], wide) ∧ @eq(loc', k)
note as channels
  channel add : k
  channel open : k
end note
@enduml
`,
		},
		"a diagram whose clauses an author bound with @exists (printed back as written)": {
			Args: []string{},
			Stdin: `@startuml
state "s0" as s0
s0 : n ; N
[*] --> s0 : @exists(x : N) (@eq(n', x))
s0 --> s0 : pick ; @exists(y : N) (@ne(n, y)) ; @exists(y : N) (@exists(z : N) (@eq(n', y) ∧ @ne(y, z)))
@enduml
`,
			Want: `@startuml
state "s0" as s0
s0 : n ; N
[*] --> s0 : @exists(x : N) (@eq(n', x))
s0 --> s0 : pick ; @exists(y : N) (@ne(n, y)) ; @exists(y : N) (@exists(z : N) (@eq(n', y) ∧ @ne(y, z)))
@enduml
`,
		},
		"-v (representative value)": {
			Args: []string{"-v"},
			Want: version.Version + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(testCase.Want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncRefuses(t *testing.T) {
	type testCase struct {
		Args       []string
		Stdin      string
		WantStderr string
	}

	testCases := map[string]testCase{
		"@opaque": {
			Args:       []string{filepath.Join("testdata", "old_opaque.puml")},
			WantStderr: "unknown tool word @opaque",
		},
		"a renaming @[x := 1]": {
			Args:       []string{filepath.Join("testdata", "old_update.puml")},
			WantStderr: "unknown tool word @ at 0",
		},
		"text that is not a diagram": {
			Args:       []string{},
			Stdin:      "not a diagram\n",
			WantStderr: "expected @startuml at line 1, col 1",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 1 {
				t.Errorf("want 1, got %d", exitStatus)
			}
			if !strings.Contains(spy.Stderr.String(), testCase.WantStderr) {
				t.Errorf("want %q on stderr, got %q", testCase.WantStderr, spy.Stderr.String())
			}
			if spy.Stdout.String() != "" {
				t.Errorf("want nothing on stdout, got %q", spy.Stdout.String())
			}
		})
	}
}

// csdfsort prints a global diagram as written, so that a review diff shows
// what the author edited. It reads no included file: as a git textconv it
// gets a blob with no directory around it.
func TestNewMainFuncPrintsAGlobalDiagramAsWritten(t *testing.T) {
	// Setup
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "promote", "RECEIPTS.puml"))
	if err != nil {
		t.Fatal(err)
	}
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(string(source)))

	// Execute
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	want := `@startuml RECEIPTS
state "稼働中" as running {
  running : cycles ; Date ⇸ Cycle
  running : receipts ; ReceiptId ⇸ Receipt
  state "cycles : (基準日 : Date) ⇸ Cycle" as running_cycles <<promote>> {
    !include local/CYCLE.puml
  }
  state "receipts : (入荷ID : ReceiptId) ⇸ Receipt" as running_receipts <<promote>> {
    !include local/RECEIPT.puml
  }
}
[*] --> running
note as sync_LOG
  sync LOG : receipts(入荷ID), cycles(基準日)
end note
note as constrain1
  constrain LOG?入荷ID?基準日?数量 ; $基準日 は $入荷ID の入荷日である
end note
note as constrain2
  constrain RECEIVE?入荷ID?数量 ; $数量 は箱入数の倍数である
end note
@enduml
`
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
