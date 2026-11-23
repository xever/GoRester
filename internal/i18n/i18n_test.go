package i18n

import "testing"

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		name   string
		pref   string
		system []string
		want   Language
	}{
		{"saved Chinese wins", "zh", []string{"en-US"}, Chinese},
		{"saved English wins", "en", []string{"zh-Hans-CN"}, English},
		{"system Chinese first", "", []string{"zh-Hans-CN", "en-US"}, Chinese},
		{"system English first", "", []string{"en-US", "zh-Hans-CN"}, English},
		{"BCP47 Chinese prefix", "", []string{"zh-CN"}, Chinese},
		{"unknown system falls back", "", []string{"ja-JP", "fr-FR"}, Chinese},
		{"no system preference falls back", "", nil, Chinese},
		{"unknown saved value follows system", "de", []string{"en-US"}, English},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.pref, tt.system); got != tt.want {
				t.Fatalf("Resolve(%q, %v) = %q, want %q", tt.pref, tt.system, got, tt.want)
			}
		})
	}
}

func TestTUsesCurrentLanguageAndReportsMissingKeys(t *testing.T) {
	old := Current()
	t.Cleanup(func() { Set(old) })

	Set(Chinese)
	if got := T("home.title"); got != "我的项目" {
		t.Fatalf("Chinese T = %q", got)
	}
	Set(English)
	if got := T("home.title"); got != "My Projects" {
		t.Fatalf("English T = %q", got)
	}
	if got := T("missing.key"); got != "missing.key" {
		t.Fatalf("missing key = %q, want the key itself", got)
	}
}

func TestBundlesHaveTheSameKeys(t *testing.T) {
	if len(bundles[Chinese]) != len(bundles[English]) {
		t.Fatalf("language bundles differ: zh=%d en=%d", len(bundles[Chinese]), len(bundles[English]))
	}
	for key := range bundles[Chinese] {
		if _, ok := bundles[English][key]; !ok {
			t.Errorf("en missing key %q", key)
		}
	}
	for key := range bundles[English] {
		if _, ok := bundles[Chinese][key]; !ok {
			t.Errorf("zh missing key %q", key)
		}
	}
}
