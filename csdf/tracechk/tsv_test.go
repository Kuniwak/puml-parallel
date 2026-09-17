package tracechk_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func TestReadTSVReadsOneEventPerRowUnderAnEventHeader(t *testing.T) {
	// Arrange
	input := "event\ninsert(coin)\nshowPurchasable(purchasableProducts)\n"

	// Act
	got, err := tracechk.ReadTSV(strings.NewReader(input))

	// Assert
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	want := []csdf.Event{"insert(coin)", "showPurchasable(purchasableProducts)"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestReadTSVRejectsMalformedInput(t *testing.T) {
	testCases := map[string]string{
		"missing header":     "insert(coin)\n",
		"empty input":        "",
		"tau is not visible": "event\ntau\n",
		"two columns":        "event\ninsert(coin)\tfoo\n",
	}

	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got, err := tracechk.ReadTSV(strings.NewReader(input))

			// Assert
			if err == nil {
				t.Errorf("want an error, got %v", got)
			}
		})
	}
}
