package config

import "testing"

func TestRemoteBotsAllowed_EmptyAllowlist(t *testing.T) {
	f := FeaturesConfig{}
	if !f.RemoteBotsAllowed("") || !f.RemoteBotsAllowed("BD") {
		t.Fatal("empty allowlist should allow all")
	}
}

func TestRemoteBotsAllowed_Restricted(t *testing.T) {
	f := FeaturesConfig{RemoteBotsRegions: []string{"BD", "US"}}
	if !f.RemoteBotsAllowed("bd") {
		t.Fatal("BD should be allowed")
	}
	if f.RemoteBotsAllowed("DE") {
		t.Fatal("DE should be denied")
	}
	if f.RemoteBotsAllowed("") {
		t.Fatal("unknown country should be denied when allowlist set")
	}
}
