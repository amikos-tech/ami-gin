package gin

import "testing"

func TestPropertyTestMinSuccessfulTestsForMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		short       bool
		normal      int
		shortBudget int
		want        int
	}{
		{name: "manual run keeps full budget", short: false, normal: 1000, shortBudget: 100, want: 1000},
		{name: "short run uses lower budget", short: true, normal: 1000, shortBudget: 100, want: 100},
		{name: "short run ignores zero short budget", short: true, normal: 1000, shortBudget: 0, want: 1000},
		{name: "short run ignores larger short budget", short: true, normal: 100, shortBudget: 250, want: 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := propertyTestMinSuccessfulTestsForMode(tt.short, tt.normal, tt.shortBudget); got != tt.want {
				t.Fatalf("propertyTestMinSuccessfulTestsForMode(%t, %d, %d) = %d, want %d", tt.short, tt.normal, tt.shortBudget, got, tt.want)
			}
		})
	}
}
