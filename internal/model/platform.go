package model

// RemotePlatforms are the only problem sources executed by a remote online
// judge. Every other source — "local", AI-generated problems ("ai"), and any
// future one — is compiled and run by the local judge worker pool.
//
// This list is the single source of truth. The worker claims a submission when
// its problem source is NOT in here; the vjudge submit worker takes it when it
// IS. Both used to be derived from `source != "local"`, which put every
// submission to an AI problem into the remote pipeline: no platform named "ai"
// exists, so it failed with a System Error carrying zero test-case details,
// while the local worker refused to claim it at all.
//
// Adding a platform means adding it here — nothing else needs to change.
var RemotePlatforms = []string{"codeforces", "atcoder", "cses", "toph", "qoj"}

// IsRemotePlatform reports whether source names a remote online judge.
func IsRemotePlatform(source string) bool {
	for _, p := range RemotePlatforms {
		if source == p {
			return true
		}
	}
	return false
}
