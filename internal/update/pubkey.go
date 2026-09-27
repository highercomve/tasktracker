package update

import (
	"encoding/base64"
	"fmt"
	"strings"

	"aead.dev/minisign"
)

// publicKeyBase64 is the updater's minisign public key (key ID 1F96A2977EF37E6B),
// base64-encoded like Tauri's updater pubkey. The matching private key lives
// only in the UPDATER_PRIVATE_KEY secret used by the release workflow.
const publicKeyBase64 = "dW50cnVzdGVkIGNvbW1lbnQ6IG1pbmlzaWduIHB1YmxpYyBrZXk6IDFGOTZBMjk3N0VGMzdFNkIKUldScmZ2TitsNktXSHkzaEdBbmRFVU14bFZqZVVRamVXcCtXYWdpVGFoSitWQjkxUiszVmM2MVUK"

// publicKey is swapped out by tests.
var publicKey = PublicKey

// PublicKey returns the key update bundles must be signed with.
func PublicKey() (minisign.PublicKey, error) {
	var pub minisign.PublicKey
	text, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return pub, fmt.Errorf("invalid updater public key: %w", err)
	}
	// Drop the "untrusted comment:" line of the key file.
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if err := pub.UnmarshalText([]byte(lines[len(lines)-1])); err != nil {
		return pub, fmt.Errorf("invalid updater public key: %w", err)
	}
	return pub, nil
}
