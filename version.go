package main

// Version and GitHash are injected at build time via ldflags:
//
//	-X main.Version=X.Y.Z -X main.GitHash=<short-hash>
var Version = "0.2.0"
var GitHash = ""

func versionString() string {
	if GitHash != "" {
		return Version + " (" + GitHash + ")"
	}
	return Version
}
