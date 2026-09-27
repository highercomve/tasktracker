// Command updater signs release bundles and writes the latest.json manifest
// read by tasktracker's self-updater (see internal/update).
//
//	UPDATER_PRIVATE_KEY=<base64 minisign key> go run ./tools/updater \
//	    --version v0.2.0 --base-url https://github.com/highercomve/tasktracker/releases/download/v0.2.0 \
//	    --changelog CHANGELOG.md --out latest.json artifacts/*
//
// Files that are not update bundles are ignored, so passing every release
// asset is fine. Each bundle also gets a <name>.sig file next to it.
//
// To create a new signing key pair (or `make updater-key`):
//
//	go run ./tools/updater --generate-key DIR --pubkey-file internal/update/pubkey.go
//
// writes DIR/updater.key (the UPDATER_PRIVATE_KEY secret, base64-encoded) and
// puts the public key in internal/update/pubkey.go (without --pubkey-file it
// is printed instead).
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aead.dev/minisign"

	"github.com/highercomve/tasktracker/internal/update"
)

// tauriArch maps the Go architecture in asset names to Tauri's naming.
var tauriArch = map[string]string{"amd64": "x86_64", "arm64": "aarch64"}

// bundle patterns map release asset names to manifest platform keys. The
// first key is tasktracker's own ("<os>-<arch>-<kind>"); the plain
// "<os>-<arch>" key keeps the manifest readable by Tauri-style clients. The
// Flatpak tarballs don't match: Flatpak installs are updated by Flatpak.
var bundles = []struct {
	pattern *regexp.Regexp
	keys    func(arch string) []string
}{
	{regexp.MustCompile(`^tasktracker-linux-(amd64|arm64)\.tar\.xz$`),
		func(a string) []string { return []string{"linux-" + a + "-tar", "linux-" + a} }},
	{regexp.MustCompile(`^tasktracker-windows-(amd64|arm64)\.zip$`),
		func(a string) []string { return []string{"windows-" + a + "-zip", "windows-" + a} }},
	{regexp.MustCompile(`^tasktracker-darwin-(amd64|arm64)\.zip$`),
		func(a string) []string { return []string{"darwin-" + a + "-app", "darwin-" + a} }},
}

func main() {
	version := flag.String("version", "", "release version, e.g. v0.0.12")
	baseURL := flag.String("base-url", "", "URL the release assets are downloaded from")
	changelog := flag.String("changelog", "", "CHANGELOG.md to take the release notes from")
	out := flag.String("out", "latest.json", "manifest to write")
	genKey := flag.String("generate-key", "", "write a new key pair's private key to this directory and print the public key")
	pubkeyFile := flag.String("pubkey-file", "", "with --generate-key: also put the public key in this Go file (internal/update/pubkey.go)")
	flag.Parse()
	if *genKey != "" {
		generateKey(*genKey, *pubkeyFile)
		return
	}
	if *version == "" || *baseURL == "" {
		fail("--version and --base-url are required")
	}

	key, err := loadKey(os.Getenv("UPDATER_PRIVATE_KEY"))
	if err != nil {
		fail(err.Error())
	}
	pub, _ := key.Public().(minisign.PublicKey)
	if want, err := update.PublicKey(); err != nil || want.ID() != pub.ID() {
		fail(fmt.Sprintf("UPDATER_PRIVATE_KEY (key %X) does not match the public key built into tasktracker", pub.ID()))
	}

	m := update.Manifest{
		Version:   strings.TrimPrefix(*version, "v"),
		PubDate:   time.Now().UTC().Format(time.RFC3339),
		Platforms: map[string]update.Platform{},
	}
	if *changelog != "" {
		m.Notes = releaseNotes(*changelog, *version)
	}

	for _, file := range flag.Args() {
		name := filepath.Base(file)
		for _, b := range bundles {
			sub := b.pattern.FindStringSubmatch(name)
			if sub == nil {
				continue
			}
			sig, err := sign(key, file, name, m.Version)
			if err != nil {
				fail(err.Error())
			}
			encoded := base64.StdEncoding.EncodeToString(sig)
			if err := os.WriteFile(file+".sig", []byte(encoded), 0o644); err != nil {
				fail(err.Error())
			}
			for _, k := range b.keys(tauriArch[sub[1]]) {
				m.Platforms[k] = update.Platform{Signature: encoded, URL: strings.TrimSuffix(*baseURL, "/") + "/" + name}
			}
			fmt.Printf("signed %s\n", name)
		}
	}
	if len(m.Platforms) == 0 {
		fail("no update bundles among the given files")
	}

	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		fail(err.Error())
	}
	keys := make([]string, 0, len(m.Platforms))
	for k := range m.Platforms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("wrote %s for %s: %s\n", *out, m.Version, strings.Join(keys, ", "))
}

// loadKey accepts the base64-encoded minisign key file (Tauri's format) or the
// key file itself.
func loadKey(s string) (minisign.PrivateKey, error) {
	var key minisign.PrivateKey
	s = strings.TrimSpace(s)
	if s == "" {
		return key, fmt.Errorf("UPDATER_PRIVATE_KEY is not set")
	}
	text := []byte(s)
	if !strings.Contains(s, "\n") {
		if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
			text = decoded
		}
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if err := key.UnmarshalText([]byte(lines[len(lines)-1])); err != nil {
		return key, fmt.Errorf("reading UPDATER_PRIVATE_KEY: %w", err)
	}
	return key, nil
}

// sign produces a prehashed minisign signature whose trusted comment binds
// the file name and version (checked by the updater).
func sign(key minisign.PrivateKey, file, name, version string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := minisign.NewReader(f)
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	trusted := fmt.Sprintf("timestamp:%d file:%s version:%s", time.Now().Unix(), name, version)
	untrusted := "signature from tasktracker updater key " + strings.ToUpper(strconv.FormatUint(key.ID(), 16))
	return r.SignWithComments(key, trusted, untrusted), nil
}

// releaseNotes extracts the version's section from a git-chglog CHANGELOG.
func releaseNotes(path, version string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	start := bytes.Index(data, []byte(`<a name="`+version+`"></a>`))
	if start < 0 {
		return ""
	}
	section := data[start:]
	if end := bytes.Index(section[1:], []byte(`<a name="`)); end >= 0 {
		section = section[:end+1]
	}
	var lines []string
	for _, l := range strings.Split(string(section), "\n") {
		// Keep the bullets and group headings, drop the anchor, title and date.
		if strings.HasPrefix(l, "<a ") || strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "> ") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// generateKey creates a key pair. The private key is written, base64-encoded
// like Tauri's, to dir/updater.key and never printed. An existing key file is
// never overwritten. With pubkeyFile, the public key is written into it.
func generateKey(dir, pubkeyFile string) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		fail(err.Error())
	}
	id := strings.ToUpper(strconv.FormatUint(pub.ID(), 16))
	privText, err := priv.MarshalText()
	if err != nil {
		fail(err.Error())
	}
	privFile := string(privText) + "\n"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail(err.Error())
	}
	path := filepath.Join(dir, "updater.key")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail(fmt.Sprintf("%v (move the existing key away first; releases signed with it need it)", err))
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString([]byte(privFile))); err != nil {
		fail(err.Error())
	}
	if err := f.Close(); err != nil {
		fail(err.Error())
	}
	pubText, _ := pub.MarshalText()
	pubB64 := base64.StdEncoding.EncodeToString([]byte(string(pubText) + "\n"))
	fmt.Printf("key ID %s\nprivate key (UPDATER_PRIVATE_KEY secret): %s\n", id, path)

	if pubkeyFile == "" {
		fmt.Printf("public key for internal/update/pubkey.go:\n%s\n", pubB64)
		return
	}
	src, err := os.ReadFile(pubkeyFile)
	if err != nil {
		fail(err.Error())
	}
	constRe := regexp.MustCompile(`const publicKeyBase64 = "[^"]*"`)
	if !constRe.Match(src) {
		fail(pubkeyFile + " has no publicKeyBase64 constant")
	}
	src = constRe.ReplaceAll(src, []byte(`const publicKeyBase64 = "`+pubB64+`"`))
	src = regexp.MustCompile(`\(key ID [0-9A-F]+\)`).ReplaceAll(src, []byte("(key ID "+id+")"))
	if err := os.WriteFile(pubkeyFile, src, 0o644); err != nil {
		fail(err.Error())
	}
	fmt.Printf("public key written to %s\n", pubkeyFile)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "updater:", msg)
	os.Exit(1)
}
