package main

import "testing"

func TestRootPackageIsInstallableMain(t *testing.T) {
	if binaryName != "email-cli" {
		t.Fatalf("binaryName = %q, want %q", binaryName, "email-cli")
	}
}
