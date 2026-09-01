package csdfhelpcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goreleaserPath = "../../../.goreleaser.yaml"

func registeredNames(t *testing.T) map[string]bool {
	t.Helper()

	names := make(map[string]bool)
	for _, entry := range Registry() {
		names[entry.Name] = true
	}
	return names
}

// TestRegistryCoversEveryTool guards against adding a tool under tools/ without
// registering it here. Every directory holding a main.go is a shipped binary.
func TestRegistryCoversEveryTool(t *testing.T) {
	// Arrange
	dirEntries, err := os.ReadDir("../..")
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}
	registered := registeredNames(t)

	// Act & Assert
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join("../..", dirEntry.Name(), "main.go")); err != nil {
			continue
		}
		if !registered[dirEntry.Name()] {
			t.Errorf("tools/%s is not in Registry", dirEntry.Name())
		}
	}
}

// TestRegistryMatchesGoreleaser keeps the registry and the release manifest in
// step, so a tool cannot be documented without being shipped or the reverse.
func TestRegistryMatchesGoreleaser(t *testing.T) {
	// Arrange
	binaries, archived := readGoreleaser(t)
	registered := registeredNames(t)

	// Act & Assert
	for name := range registered {
		if !binaries[name] {
			t.Errorf("%s is in Registry but has no build in .goreleaser.yaml", name)
		}
		if !archived[name] {
			t.Errorf("%s is in Registry but is not in the archive of .goreleaser.yaml", name)
		}
	}
	for name := range binaries {
		if !registered[name] {
			t.Errorf("%s is built by .goreleaser.yaml but is not in Registry", name)
		}
	}
}

// readGoreleaser returns the binaries built by .goreleaser.yaml and the
// binaries gathered into its archive. It scans the file line by line rather
// than parsing YAML, which the module does not depend on; a layout change
// empties the maps and fails the test rather than passing silently.
func readGoreleaser(t *testing.T) (binaries, archived map[string]bool) {
	t.Helper()

	bs, err := os.ReadFile(goreleaserPath)
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	binaryByID := make(map[string]string)
	var archivedIDs []string
	section := ""
	buildID := ""
	inIDs := false
	for _, line := range strings.Split(string(bs), "\n") {
		if key, ok := strings.CutSuffix(line, ":"); ok && !strings.HasPrefix(line, " ") {
			section, buildID, inIDs = key, "", false
			continue
		}

		switch section {
		case "builds":
			if id, ok := strings.CutPrefix(line, "  - id: "); ok {
				buildID = strings.TrimSpace(id)
			}
			if binary, ok := strings.CutPrefix(line, "    binary: "); ok {
				binaryByID[buildID] = strings.TrimSpace(binary)
			}
		case "archives":
			if strings.TrimSpace(line) == "ids:" {
				inIDs = true
				continue
			}
			if !inIDs {
				continue
			}
			id, ok := strings.CutPrefix(line, "      - ")
			if !ok {
				inIDs = false
				continue
			}
			archivedIDs = append(archivedIDs, strings.TrimSpace(id))
		}
	}

	binaries = make(map[string]bool, len(binaryByID))
	for id, binary := range binaryByID {
		if id == "" {
			t.Errorf("want an id for the build of %s, got none", binary)
		}
		binaries[binary] = true
	}

	archived = make(map[string]bool, len(archivedIDs))
	for _, id := range archivedIDs {
		binary, ok := binaryByID[id]
		if !ok {
			t.Errorf("%s is archived by %s but has no build", id, goreleaserPath)
			continue
		}
		archived[binary] = true
	}

	if len(binaries) == 0 {
		t.Fatalf("want builds in %s, got none", goreleaserPath)
	}
	if len(archived) == 0 {
		t.Fatalf("want archived ids in %s, got none", goreleaserPath)
	}
	return binaries, archived
}

func TestRegistryEntriesAreComplete(t *testing.T) {
	// Arrange & Act
	entries := Registry()

	// Assert
	for _, entry := range entries {
		if entry.Summary == "" {
			t.Errorf("%s: want non-empty Summary, got empty", entry.Name)
		}
		if entry.Run == nil {
			t.Errorf("%s: want non-nil Run, got nil", entry.Name)
		}
	}
}
