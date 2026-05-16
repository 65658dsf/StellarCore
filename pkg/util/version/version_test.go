package version

import "testing"

func TestCoreFromFull(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "stellar core full version",
			in:   "StellarCore-1.1.6-0.67.0",
			want: "1.1.6",
		},
		{
			name: "unknown format is unchanged",
			in:   "dev",
			want: "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CoreFromFull(tt.in); got != tt.want {
				t.Fatalf("CoreFromFull(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
