package pngsrc

import "testing"

func TestExtract(t *testing.T) {
	cases := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{
			name:  "plain text passes through unchanged",
			input: []byte("@startuml\n@enduml"),
			want:  "@startuml\n@enduml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Extract: want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extract: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Extract: got %q, want %q", got, tc.want)
			}
		})
	}
}
