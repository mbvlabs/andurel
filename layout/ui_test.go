package layout

import "testing"

func TestParseUICombo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		combo   string
		want    UISelection
		wantErr bool
	}{
		{
			name:  "empty defaults to react/pnpm",
			combo: "",
			want:  UISelection{Inertia: "react", PackageManager: "pnpm"},
		},
		{
			name:  "react/pnpm",
			combo: "react/pnpm",
			want:  UISelection{Inertia: "react", PackageManager: "pnpm"},
		},
		{
			name:  "vue/bun",
			combo: "vue/bun",
			want:  UISelection{Inertia: "vue", PackageManager: "bun"},
		},
		{
			name:  "svelte/npm",
			combo: "svelte/npm",
			want:  UISelection{Inertia: "svelte", PackageManager: "npm"},
		},
		{
			name:  "templ/datastar",
			combo: "templ/datastar",
			want:  UISelection{},
		},
		{
			name:    "bare react rejected",
			combo:   "react",
			wantErr: true,
		},
		{
			name:    "bare datastar rejected",
			combo:   "datastar",
			wantErr: true,
		},
		{
			name:    "yarn rejected",
			combo:   "react/yarn",
			wantErr: true,
		},
		{
			name:    "unknown adapter",
			combo:   "angular/pnpm",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUICombo(tt.combo)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUICombo: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			if got.IsInertia() != (tt.want.Inertia != "") {
				t.Fatalf("IsInertia = %v", got.IsInertia())
			}
			if got.IsDatastar() != (tt.want.Inertia == "") {
				t.Fatalf("IsDatastar = %v", got.IsDatastar())
			}
		})
	}
}
