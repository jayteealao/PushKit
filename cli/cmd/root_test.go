package cmd

import "testing"

func TestResolveCredentials(t *testing.T) {
	tests := []struct {
		name    string
		cfgURL  string
		cfgKey  string
		envURL  string
		envKey  string
		flagURL string
		flagKey string
		wantURL string
		wantKey string
	}{
		{
			name:    "config only",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			wantURL: "http://config.example.com",
			wantKey: "cfg-key",
		},
		{
			name:    "env overrides config",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			envURL:  "http://env.example.com",
			envKey:  "env-key",
			wantURL: "http://env.example.com",
			wantKey: "env-key",
		},
		{
			name:    "flag overrides env",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			envURL:  "http://env.example.com",
			envKey:  "env-key",
			flagURL: "http://flag.example.com",
			flagKey: "flag-key",
			wantURL: "http://flag.example.com",
			wantKey: "flag-key",
		},
		{
			name:    "flag overrides config (no env)",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			flagURL: "http://flag.example.com",
			flagKey: "flag-key",
			wantURL: "http://flag.example.com",
			wantKey: "flag-key",
		},
		{
			name:    "missing values stay empty",
			wantURL: "",
			wantKey: "",
		},
		{
			name:    "partial override: env url only",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			envURL:  "http://env.example.com",
			wantURL: "http://env.example.com",
			wantKey: "cfg-key",
		},
		{
			name:    "partial override: flag key only",
			cfgURL:  "http://config.example.com",
			cfgKey:  "cfg-key",
			flagKey: "flag-key",
			wantURL: "http://config.example.com",
			wantKey: "flag-key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotKey := resolveCredentials(
				tc.cfgURL, tc.cfgKey,
				tc.envURL, tc.envKey,
				tc.flagURL, tc.flagKey,
			)
			if gotURL != tc.wantURL {
				t.Errorf("url: got %q, want %q", gotURL, tc.wantURL)
			}
			if gotKey != tc.wantKey {
				t.Errorf("key: got %q, want %q", gotKey, tc.wantKey)
			}
		})
	}
}
