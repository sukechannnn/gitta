package commands

import "testing"

func TestMapDisplayToOriginalIdxSkipsNewFileHeader(t *testing.T) {
	diff := "diff --git a/new.txt b/new.txt\nnew file mode 100644\nindex 0000000..1111111\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1,2 @@\n+one\n+two\n"
	mapping := MapDisplayToOriginalIdx(diff)
	// Display row 0 is "+one", line 6 of the diff text
	if mapping[0] != 6 || mapping[1] != 7 {
		t.Errorf("MapDisplayToOriginalIdx() = %v, want 0->6, 1->7", mapping)
	}
}
