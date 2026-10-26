package csdfpromotecmd

import "testing"

func TestParseOutput(t *testing.T) {
	cases := map[string]struct {
		name    string
		want    Output
		wantErr bool
	}{
		"puml":    {name: OutputNamePUML, want: OutputPUML},
		"json":    {name: OutputNameJSON, want: OutputJSON},
		"unknown": {name: "tsv", wantErr: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseOutput(c.name)
			if (err != nil) != c.wantErr {
				t.Fatalf("want an error %t, got %v", c.wantErr, err)
			}
			if got != c.want {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}
