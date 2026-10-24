package notification

import (
	"crypto/rand"
	"encoding/json"
)

// A destination binds the adapter, credential, recipient and verification epoch.
// Old token-only fingerprints intentionally cannot authorize delayed delivery.
// Only the digest is persisted in outboxes; it is never exposed by the API.
func channelFingerprint(ch storedChannel) string {
	material, _ := json.Marshal(struct {
		Version     int
		Provider    Provider
		Credential  string
		ChatID      int64
		Binding     string
		MetaBinding string
	}{1, ch.Provider, tokenFingerprint(ch.Credential), ch.Meta.ChatID, ch.Binding, ch.Meta.Binding})
	return tokenFingerprint(string(material))
}

func legacyDestination(provider Provider, credential string, meta channelState) string {
	return channelFingerprint(storedChannel{Provider: provider, Credential: credential, Meta: meta})
}

func newDestinationBinding() string { return tokenFingerprint(rand.Text()) }
