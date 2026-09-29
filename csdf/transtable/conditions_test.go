package transtable_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/transtable"
)

func TestParseConditions(t *testing.T) {
	type testCase struct {
		Name    string
		Want    transtable.Conditions
		WantErr bool
	}

	testCases := map[string]testCase{
		"full":     {Name: transtable.ConditionsNameFull, Want: transtable.ConditionsFull},
		"enabling": {Name: transtable.ConditionsNameEnabling, Want: transtable.ConditionsEnabling},
		"unknown":  {Name: "guards", WantErr: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got, err := transtable.ParseConditions(testCase.Name)

			// Assert
			if (err != nil) != testCase.WantErr {
				t.Fatalf("want an error %t, got %v", testCase.WantErr, err)
			}
			if got != testCase.Want {
				t.Errorf("want %v, got %v", testCase.Want, got)
			}
		})
	}
}
