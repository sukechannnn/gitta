package util

// SplitLines splits a string by newline characters
func SplitLines(input string) []string {
	lines := []string{}
	currentLine := ""
	for _, r := range input {
		if r == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(r)
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}
