package util

import (
	"reflect"
	"testing"
)

func TestDiffHeaderMask(t *testing.T) {
	lines := []string{
		"diff --git a/new.go b/new.go",
		"new file mode 100644",
		"index 0000000..3e75765",
		"--- /dev/null",
		"+++ b/new.go",
		"@@ -0,0 +1,2 @@",
		"+package main",
		"+",
		"diff --git a/old.go b/renamed.go",
		"similarity index 90%",
		"rename from old.go",
		"rename to renamed.go",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}
	want := []bool{
		true, true, true, true, true, true, false, false,
		true, true, true, true, true, false, false,
	}
	if got := DiffHeaderMask(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("DiffHeaderMask() = %v, want %v", got, want)
	}
}

func TestDiffHeaderMaskWithoutGitHeader(t *testing.T) {
	// Messages such as the binary notice are shown as they are
	lines := []string{"Binary file changed", "", "--- a/f", "+++ b/f", "@@ -1 +1 @@", " ctx"}
	want := []bool{false, false, true, true, true, false}
	if got := DiffHeaderMask(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("DiffHeaderMask() = %v, want %v", got, want)
	}
}
