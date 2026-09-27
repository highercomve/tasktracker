package main

import "testing"

func TestBundleKeys(t *testing.T) {
	want := map[string]string{
		"tasktracker-linux-amd64.tar.xz":         "linux-x86_64-tar",
		"tasktracker-linux-arm64.tar.xz":         "linux-aarch64-tar",
		"tasktracker-windows-amd64.zip":          "windows-x86_64-zip",
		"tasktracker-darwin-arm64.zip":           "darwin-aarch64-app",
		"tasktracker-linux-amd64-flatpak.tar.xz": "",
		"tasktracker-linux-amd64.tar.xz.sig":     "",
		"latest.json":                            "",
	}
	for name, key := range want {
		got := ""
		for _, b := range bundles {
			if sub := b.pattern.FindStringSubmatch(name); sub != nil {
				got = b.keys(tauriArch[sub[1]])[0]
			}
		}
		if got != key {
			t.Errorf("%s: key %q, want %q", name, got, key)
		}
	}
}
