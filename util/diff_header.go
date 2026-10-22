package util

import "strings"

// DiffHeaderMask reports, for each diff line, whether it is a header that is
// not shown: hunk headers, and the file header from "diff --git" up to the
// first hunk, including extended lines such as "new file mode" or "rename
// from". "index", "---" and "+++" lines also count when there is no
// "diff --git" line before them.
func DiffHeaderMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inFileHeader := false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			inFileHeader = true
			mask[i] = true
		case strings.HasPrefix(line, "@@"):
			inFileHeader = false
			mask[i] = true
		case inFileHeader,
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "--- "),
			strings.HasPrefix(line, "+++ "):
			mask[i] = true
		}
	}
	return mask
}
