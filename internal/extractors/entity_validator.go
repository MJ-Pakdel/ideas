package extractors

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/example/idaes/internal/types"
)

// EntityValidator handles validation and filtering of extracted entities
type EntityValidator struct {
	StrictMode          bool
	MinConfidence       float64
	MaxEntities         int
	EnableDeduplication bool
	ValidateOffsets     bool
	ValidateTypes       bool
	AllowedTypes        map[types.EntityType]bool
}

// ValidationResult contains the results of entity validation
type ValidationResult struct {
	IsValid         bool
	Errors          []string
	Warnings        []string
	FixedIssues     []string
	EntityCount     int
	DuplicatesFound int
	InvalidEntities int
}

// NewEntityValidator creates a new entity validator with default settings
func NewEntityValidator() *EntityValidator {
	return &EntityValidator{
		StrictMode:          false,
		MinConfidence:       0.0,
		MaxEntities:         100,
		EnableDeduplication: true,
		ValidateOffsets:     true,
		ValidateTypes:       true,
		AllowedTypes:        makeAllowedTypesMap(types.GetEntityTypesList()),
	}
}

// WithStrictMode enables or disables strict validation mode
func (ev *EntityValidator) WithStrictMode(enabled bool) *EntityValidator {
	ev.StrictMode = enabled
	return ev
}

// WithMinConfidence sets the minimum confidence threshold
func (ev *EntityValidator) WithMinConfidence(min float64) *EntityValidator {
	ev.MinConfidence = min
	return ev
}

// WithMaxEntities sets the maximum number of entities allowed
func (ev *EntityValidator) WithMaxEntities(max int) *EntityValidator {
	ev.MaxEntities = max
	return ev
}

// WithAllowedTypes sets the allowed entity types
func (ev *EntityValidator) WithAllowedTypes(entityTypes []types.EntityType) *EntityValidator {
	ev.AllowedTypes = makeAllowedTypesMap(entityTypes)
	return ev
}

// ValidateAndFilterEntities performs comprehensive validation and filtering
func (ev *EntityValidator) ValidateAndFilterEntities(entities []*types.ExtractedEntity, documentContent string) ([]*types.ExtractedEntity, *ValidationResult) {
	result := &ValidationResult{
		IsValid:         true,
		Errors:          make([]string, 0),
		Warnings:        make([]string, 0),
		FixedIssues:     make([]string, 0),
		EntityCount:     len(entities),
		DuplicatesFound: 0,
		InvalidEntities: 0,
	}

	if len(entities) == 0 {
		return entities, result
	}

	var validEntities []*types.ExtractedEntity

	// Step 1: Individual entity validation
	for i, entity := range entities {
		if entity == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("entity[%d]: nil entity", i))
			result.InvalidEntities++
			continue
		}

		entityResult := ev.validateSingleEntity(entity, documentContent)

		// Fix issues if possible
		if len(entityResult.FixedIssues) > 0 {
			result.FixedIssues = append(result.FixedIssues, entityResult.FixedIssues...)
		}

		// Apply fixes and check if entity is still valid
		if entityResult.IsValid || (!ev.StrictMode && ev.canAutoFix(entityResult)) {
			fixedEntity := ev.autoFixEntity(entity, entityResult)
			if fixedEntity != nil {
				validEntities = append(validEntities, fixedEntity)
			} else {
				result.InvalidEntities++
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("entity[%d]: %s", i, strings.Join(entityResult.Errors, "; ")))
			result.InvalidEntities++
		}

		if len(entityResult.Warnings) > 0 {
			for _, warning := range entityResult.Warnings {
				result.Warnings = append(result.Warnings, fmt.Sprintf("entity[%d]: %s", i, warning))
			}
		}
	}

	// Step 2: Deduplication
	if ev.EnableDeduplication && len(validEntities) > 1 {
		dedupEntities, dupCount := ev.deduplicateEntities(validEntities)
		validEntities = dedupEntities
		result.DuplicatesFound = dupCount
	}

	// Step 3: Apply limits
	if len(validEntities) > ev.MaxEntities {
		// Sort by confidence and keep the best ones
		sortedEntities := ev.sortEntitiesByConfidence(validEntities)
		validEntities = sortedEntities[:ev.MaxEntities]
		result.Warnings = append(result.Warnings, fmt.Sprintf("limited to %d entities (from %d)", ev.MaxEntities, len(entities)))
	}

	// Step 4: Final validation
	result.EntityCount = len(validEntities)
	result.IsValid = len(result.Errors) == 0 && len(validEntities) > 0

	return validEntities, result
}

// validateSingleEntity validates a single entity
func (ev *EntityValidator) validateSingleEntity(entity *types.ExtractedEntity, documentContent string) *ValidationResult {
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		FixedIssues: make([]string, 0),
	}

	// Validate required fields
	if entity.Text == "" {
		result.Errors = append(result.Errors, "entity text is empty")
		result.IsValid = false
	}

	if entity.Type == "" {
		result.Errors = append(result.Errors, "entity type is empty")
		result.IsValid = false
	}

	// Validate entity type
	if ev.ValidateTypes && entity.Type != "" {
		if !ev.AllowedTypes[entity.Type] {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid entity type: %s", entity.Type))
			result.IsValid = false
		}
	}

	// Validate confidence
	if entity.Confidence < 0.0 || entity.Confidence > 1.0 {
		if entity.Confidence < 0.0 {
			entity.Confidence = 0.0
			result.FixedIssues = append(result.FixedIssues, "confidence set to 0.0 (was negative)")
		} else {
			entity.Confidence = 1.0
			result.FixedIssues = append(result.FixedIssues, "confidence set to 1.0 (was > 1.0)")
		}
	}

	if entity.Confidence < ev.MinConfidence {
		result.Errors = append(result.Errors, fmt.Sprintf("confidence %.2f below threshold %.2f", entity.Confidence, ev.MinConfidence))
		result.IsValid = false
	}

	// Validate offsets
	if ev.ValidateOffsets && documentContent != "" {
		if entity.StartOffset < 0 {
			result.Errors = append(result.Errors, "start_offset is negative")
			result.IsValid = false
		}

		if entity.EndOffset <= entity.StartOffset {
			result.Errors = append(result.Errors, "end_offset must be greater than start_offset")
			result.IsValid = false
		}

		if entity.EndOffset > len(documentContent) {
			result.Errors = append(result.Errors, "end_offset exceeds document length")
			result.IsValid = false
		}

		// Validate that the text matches the offsets
		if entity.StartOffset >= 0 && entity.EndOffset <= len(documentContent) && entity.EndOffset > entity.StartOffset {
			extractedText := documentContent[entity.StartOffset:entity.EndOffset]
			if strings.TrimSpace(extractedText) != strings.TrimSpace(entity.Text) {
				result.Warnings = append(result.Warnings, "text does not match offsets")
			}
		}
	}

	// Type-specific validation
	if err := ev.validateEntityByType(entity); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}

	// Text quality validation
	if warnings := ev.validateTextQuality(entity.Text); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	return result
}

// validateEntityByType performs type-specific validation
func (ev *EntityValidator) validateEntityByType(entity *types.ExtractedEntity) error {
	switch entity.Type {
	case types.EntityTypeEmail:
		return ev.validateEmail(entity.Text)
	case types.EntityTypeURL:
		return ev.validateURL(entity.Text)
	case types.EntityTypeDate:
		return ev.validateDate(entity.Text)
	case types.EntityTypeMetric:
		return ev.validateMetric(entity.Text)
	default:
		return nil
	}
}

// validateEmail validates email addresses
func (ev *EntityValidator) validateEmail(text string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(text) {
		return fmt.Errorf("invalid email format: %s", text)
	}
	return nil
}

// validateURL validates URLs
func (ev *EntityValidator) validateURL(text string) error {
	urlRegex := regexp.MustCompile(`^https?://[^\s]+$`)
	if !urlRegex.MatchString(text) {
		return fmt.Errorf("invalid URL format: %s", text)
	}
	return nil
}

// validateDate validates date expressions
func (ev *EntityValidator) validateDate(text string) error {
	// Common date formats to try
	dateFormats := []string{
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02-01-2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2006",
		"January 2006",
		"Q1 2006",
		"Q2 2006",
		"Q3 2006",
		"Q4 2006",
	}

	for _, format := range dateFormats {
		if _, err := time.Parse(format, text); err == nil {
			return nil
		}
	}

	// Check for relative dates
	relativeRegex := regexp.MustCompile(`(?i)(today|yesterday|tomorrow|last|next|recent|current)`)
	if relativeRegex.MatchString(text) {
		return nil
	}

	return fmt.Errorf("unrecognized date format: %s", text)
}

// validateMetric validates metric expressions
func (ev *EntityValidator) validateMetric(text string) error {
	// Check for numbers with units
	metricRegex := regexp.MustCompile(`^[\d.,]+\s*[%$€£¥\w]*$`)
	if !metricRegex.MatchString(text) {
		return fmt.Errorf("invalid metric format: %s", text)
	}
	return nil
}

// validateTextQuality checks for text quality issues
func (ev *EntityValidator) validateTextQuality(text string) []string {
	var warnings []string

	// Check for excessive whitespace
	if strings.Contains(text, "  ") {
		warnings = append(warnings, "contains excessive whitespace")
	}

	// Check for special characters that might indicate extraction errors
	if strings.ContainsAny(text, "{}[]()") && !regexp.MustCompile(`^https?://`).MatchString(text) {
		warnings = append(warnings, "contains suspicious characters")
	}

	// Check length
	if len(text) > 200 {
		warnings = append(warnings, "text is unusually long")
	}

	if len(text) < 2 {
		warnings = append(warnings, "text is very short")
	}

	// Check for incomplete words (ending with common prefixes/suffixes)
	if strings.HasSuffix(text, "-") || strings.HasPrefix(text, "-") {
		warnings = append(warnings, "text appears incomplete")
	}

	return warnings
}

// deduplicateEntities removes duplicate entities
func (ev *EntityValidator) deduplicateEntities(entities []*types.ExtractedEntity) ([]*types.ExtractedEntity, int) {
	seen := make(map[string]*types.ExtractedEntity)
	var unique []*types.ExtractedEntity
	duplicateCount := 0

	for _, entity := range entities {
		key := ev.getDuplicationKey(entity)

		if existing, exists := seen[key]; exists {
			duplicateCount++
			// Keep the entity with higher confidence
			if entity.Confidence > existing.Confidence {
				// Replace in the unique slice
				for i, e := range unique {
					if e == existing {
						unique[i] = entity
						break
					}
				}
				seen[key] = entity
			}
		} else {
			seen[key] = entity
			unique = append(unique, entity)
		}
	}

	return unique, duplicateCount
}

// getDuplicationKey creates a key for duplicate detection
func (ev *EntityValidator) getDuplicationKey(entity *types.ExtractedEntity) string {
	normalizedText := strings.ToLower(strings.TrimSpace(entity.Text))
	return fmt.Sprintf("%s:%s", entity.Type, normalizedText)
}

// sortEntitiesByConfidence sorts entities by confidence (descending)
func (ev *EntityValidator) sortEntitiesByConfidence(entities []*types.ExtractedEntity) []*types.ExtractedEntity {
	sorted := make([]*types.ExtractedEntity, len(entities))
	copy(sorted, entities)

	// Simple bubble sort by confidence (descending)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Confidence < sorted[j+1].Confidence {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// canAutoFix determines if an entity can be automatically fixed
func (ev *EntityValidator) canAutoFix(result *ValidationResult) bool {
	// Only auto-fix if there are no critical errors and only warnings
	return len(result.Errors) == 0 && len(result.Warnings) > 0
}

// autoFixEntity attempts to automatically fix common issues
func (ev *EntityValidator) autoFixEntity(entity *types.ExtractedEntity, result *ValidationResult) *types.ExtractedEntity {
	if entity == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	fixed := &types.ExtractedEntity{
		ID:              entity.ID,
		Type:            entity.Type,
		Text:            strings.TrimSpace(entity.Text),
		NormalizedText:  entity.NormalizedText,
		Confidence:      entity.Confidence,
		StartOffset:     entity.StartOffset,
		EndOffset:       entity.EndOffset,
		Context:         entity.Context,
		ContextBefore:   entity.ContextBefore,
		ContextAfter:    entity.ContextAfter,
		Section:         entity.Section,
		EntityMetadata:  entity.EntityMetadata,
		ValidationFlags: entity.ValidationFlags,
		DocumentID:      entity.DocumentID,
		ExtractedAt:     entity.ExtractedAt,
		ExtractionID:    entity.ExtractionID,
	}

	// Auto-fix text issues
	fixed.Text = strings.TrimSpace(fixed.Text)
	fixed.Text = regexp.MustCompile(`\s+`).ReplaceAllString(fixed.Text, " ")

	// Ensure normalized text is set
	if fixed.NormalizedText == "" {
		fixed.NormalizedText = fixed.Text
	}

	// Update validation flags
	if fixed.ValidationFlags == nil {
		fixed.ValidationFlags = &types.ValidationFlags{}
	}
	fixed.ValidationFlags.IsValid = true
	fixed.ValidationFlags.ValidationWarnings = result.Warnings

	return fixed
}

// makeAllowedTypesMap creates a map of allowed entity types
func makeAllowedTypesMap(entityTypes []types.EntityType) map[types.EntityType]bool {
	allowed := make(map[types.EntityType]bool)
	for _, et := range entityTypes {
		allowed[et] = true
	}
	return allowed
}

// GetValidationSummary returns a summary of validation results
func (vr *ValidationResult) GetValidationSummary() string {
	if vr.IsValid {
		return fmt.Sprintf("Valid: %d entities, %d warnings", vr.EntityCount, len(vr.Warnings))
	}
	return fmt.Sprintf("Invalid: %d errors, %d warnings, %d invalid entities", len(vr.Errors), len(vr.Warnings), vr.InvalidEntities)
}

// HasCriticalErrors checks if there are critical validation errors
func (vr *ValidationResult) HasCriticalErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings checks if there are validation warnings
func (vr *ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}
