package extractors

import (
	"regexp"
	"strings"
	"unicode"
)

// JSONCleaner handles cleaning and preprocessing of LLM responses to extract valid JSON
type JSONCleaner struct {
	StrictMode      bool
	RemoveComments  bool
	FixCommonIssues bool
}

// NewJSONCleaner creates a new JSON cleaner with default settings
func NewJSONCleaner() *JSONCleaner {
	return &JSONCleaner{
		StrictMode:      false,
		RemoveComments:  true,
		FixCommonIssues: true,
	}
}

// CleanResponse performs comprehensive cleaning of LLM response to extract valid JSON
func (jc *JSONCleaner) CleanResponse(response string) string {
	if response == "" {
		return ""
	}

	// Step 1: Basic whitespace and format cleaning
	cleaned := jc.basicClean(response)

	// Step 2: Remove markdown code blocks
	cleaned = jc.removeMarkdownBlocks(cleaned)

	// Step 3: Remove explanatory text before/after JSON
	cleaned = jc.removeExplanatoryText(cleaned)

	// Step 4: Extract JSON object/array (only if needed)
	if !jc.ValidateJSONStructure(cleaned) {
		extracted := jc.extractJSONStructure(cleaned)
		if extracted != "" && jc.ValidateJSONStructure(extracted) {
			cleaned = extracted
		}
	}

	// Step 5: Fix common JSON issues
	if jc.FixCommonIssues {
		cleaned = jc.fixCommonJSONIssues(cleaned)
	}

	// Step 6: Remove comments if enabled
	if jc.RemoveComments {
		cleaned = jc.removeJSONComments(cleaned)
	}

	// Step 7: Final validation and cleanup
	cleaned = jc.finalCleanup(cleaned)

	return cleaned
}

// basicClean performs basic string cleaning
func (jc *JSONCleaner) basicClean(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	return text
}

// removeMarkdownBlocks removes markdown code blocks
func (jc *JSONCleaner) removeMarkdownBlocks(text string) string {
	// Remove ```json and ``` blocks
	jsonBlockRegex := regexp.MustCompile("```(?:json)?\n?([\\s\\S]*?)\n?```")
	matches := jsonBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Remove standalone ``` markers
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	return strings.TrimSpace(text)
}

// removeExplanatoryText removes common explanatory prefixes and suffixes
func (jc *JSONCleaner) removeExplanatoryText(text string) string {
	// Common prefixes to remove
	prefixes := []string{
		"Here's the JSON response:",
		"Here is the JSON response:",
		"The JSON output is:",
		"JSON response:",
		"Response:",
		"Output:",
		"Result:",
		"Here's the extracted data:",
		"Here is the extracted data:",
		"The extracted entities are:",
		"Based on the analysis:",
	}

	lowerText := strings.ToLower(text)
	for _, prefix := range prefixes {
		lowerPrefix := strings.ToLower(prefix)
		if strings.HasPrefix(lowerText, lowerPrefix) {
			text = text[len(prefix):]
			text = strings.TrimSpace(text)
			break
		}
	}

	// Common suffixes to remove
	suffixes := []string{
		"This completes the entity extraction.",
		"End of JSON response.",
		"That's the complete analysis.",
		"Hope this helps!",
	}

	lowerText = strings.ToLower(text)
	for _, suffix := range suffixes {
		lowerSuffix := strings.ToLower(suffix)
		if strings.HasSuffix(lowerText, lowerSuffix) {
			text = text[:len(text)-len(suffix)]
			text = strings.TrimSpace(text)
			break
		}
	}

	return text
}

// extractJSONStructure extracts the main JSON structure from the text
func (jc *JSONCleaner) extractJSONStructure(text string) string {
	// Find the start and end of the JSON object/array
	var start, end int = -1, -1
	var braceCount, bracketCount int
	var startedWithBrace bool

	for i, char := range text {
		switch char {
		case '{':
			if start == -1 {
				start = i
				startedWithBrace = true
			}
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 && start != -1 && startedWithBrace {
				end = i + 1
				goto found
			}
		case '[':
			if start == -1 {
				start = i
				startedWithBrace = false
			}
			bracketCount++
		case ']':
			bracketCount--
			if bracketCount == 0 && start != -1 && !startedWithBrace {
				end = i + 1
				goto found
			}
		}
	}

found:
	if start != -1 && end != -1 && end <= len(text) {
		return text[start:end]
	}

	// Fallback: use regex to extract JSON-like structure
	jsonRegex := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonRegex.FindAllString(text, -1)
	if len(matches) > 0 {
		// Return the longest match (likely the main JSON object)
		longest := matches[0]
		for _, match := range matches {
			if len(match) > len(longest) {
				longest = match
			}
		}
		return longest
	}

	// If no valid JSON structure found, return original
	return text
}

// fixCommonJSONIssues fixes common JSON formatting problems
func (jc *JSONCleaner) fixCommonJSONIssues(text string) string {
	// Fix trailing commas
	commaRegex := regexp.MustCompile(`,(\s*[}\]])`)
	text = commaRegex.ReplaceAllString(text, "$1")

	// Fix missing quotes around object keys
	keyRegex := regexp.MustCompile(`(\{|\,)\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	text = keyRegex.ReplaceAllString(text, `$1"$2":`)

	// Fix single quotes to double quotes
	text = jc.fixQuotes(text)

	// Fix escaped quotes in strings
	text = strings.ReplaceAll(text, `\"`, `"`)

	// Fix boolean values
	text = regexp.MustCompile(`:\s*True\b`).ReplaceAllString(text, ": true")
	text = regexp.MustCompile(`:\s*False\b`).ReplaceAllString(text, ": false")
	text = regexp.MustCompile(`:\s*None\b`).ReplaceAllString(text, ": null")
	text = regexp.MustCompile(`:\s*undefined\b`).ReplaceAllString(text, ": null")

	return text
}

// fixQuotes converts single quotes to double quotes while preserving string content
func (jc *JSONCleaner) fixQuotes(text string) string {
	var result strings.Builder
	inString := false
	escaped := false

	for _, char := range text {
		if escaped {
			result.WriteRune(char)
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
			result.WriteRune(char)
		case '\'':
			if !inString {
				// Outside string: convert single quote to double quote
				result.WriteRune('"')
			} else {
				// Inside string: keep as single quote but escape if needed
				result.WriteRune(char)
			}
		case '"':
			inString = !inString
			result.WriteRune(char)
		default:
			result.WriteRune(char)
		}
	}

	return result.String()
}

// removeJSONComments removes comments from JSON (non-standard but sometimes present)
func (jc *JSONCleaner) removeJSONComments(text string) string {
	// Remove single line comments
	singleLineCommentRegex := regexp.MustCompile(`//.*?$`)
	text = singleLineCommentRegex.ReplaceAllString(text, "")

	// Remove multi-line comments
	multiLineCommentRegex := regexp.MustCompile(`/\*.*?\*/`)
	text = multiLineCommentRegex.ReplaceAllString(text, "")

	return text
}

// finalCleanup performs final validation and cleanup
func (jc *JSONCleaner) finalCleanup(text string) string {
	// Remove any remaining non-JSON content at the beginning or end
	text = strings.TrimSpace(text)

	// Ensure proper JSON structure exists
	if !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
		return ""
	}

	if !strings.HasSuffix(text, "}") && !strings.HasSuffix(text, "]") {
		return ""
	}

	// Remove any null bytes or other problematic characters
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, text)

	return text
}

// ExtractMultipleJSONObjects extracts multiple JSON objects from text
func (jc *JSONCleaner) ExtractMultipleJSONObjects(text string) []string {
	var objects []string

	// Clean the text first
	cleaned := jc.basicClean(text)
	cleaned = jc.removeMarkdownBlocks(cleaned)

	// Find all JSON objects
	jsonRegex := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonRegex.FindAllString(cleaned, -1)

	for _, match := range matches {
		cleaned := jc.CleanResponse(match)
		if cleaned != "" {
			objects = append(objects, cleaned)
		}
	}

	return objects
}

// ValidateJSONStructure performs basic validation of JSON structure
func (jc *JSONCleaner) ValidateJSONStructure(text string) bool {
	if text == "" {
		return false
	}

	// Basic structure check
	if !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
		return false
	}

	if !strings.HasSuffix(text, "}") && !strings.HasSuffix(text, "]") {
		return false
	}

	// Count braces and brackets
	braceCount := 0
	bracketCount := 0
	inString := false
	escaped := false

	for _, char := range text {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
			case '[':
				bracketCount++
			case ']':
				bracketCount--
			}
		}
	}

	return braceCount == 0 && bracketCount == 0
}

// GetCleaningStats returns statistics about the cleaning process
type CleaningStats struct {
	OriginalLength  int
	CleanedLength   int
	MarkdownRemoved bool
	JSONExtracted   bool
	IssuesFixed     int
	CommentsRemoved int
}

// CleanResponseWithStats performs cleaning and returns statistics
func (jc *JSONCleaner) CleanResponseWithStats(response string) (string, *CleaningStats) {
	stats := &CleaningStats{
		OriginalLength: len(response),
	}

	cleaned := response

	// Track each step
	if strings.Contains(cleaned, "```") {
		stats.MarkdownRemoved = true
	}

	cleaned = jc.CleanResponse(cleaned)

	stats.CleanedLength = len(cleaned)
	stats.JSONExtracted = len(cleaned) > 0 && len(cleaned) < len(response)

	return cleaned, stats
}
