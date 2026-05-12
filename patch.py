import os

filepath = 'internal/core/kernel_validation.go'
with open(filepath, 'r', encoding='utf-8') as f:
    content = f.read()

# 1. ValidateLearnedRule
content = content.replace(
"""	if k.schemaValidator == nil {
		// Validator not initialized - allow (defensive)
		return nil
	}""",
"""	if k.schemaValidator == nil {
		return fmt.Errorf("schema validator uninitialized")
	}""")

# 2. ValidateLearnedRules
content = content.replace(
"""func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}""",
"""func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return []error{fmt.Errorf("schema validator uninitialized")}
	}""")

# 3. ValidateLearnedProgram
content = content.replace(
"""func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}""",
"""func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return fmt.Errorf("schema validator uninitialized")
	}""")

# 4. Atomic Write
content = content.replace(
"""		// Persist healed rules back to disk if we have a file path
		if filePath != "" {
			if err := os.WriteFile(filePath, []byte(result.healedText), 0644); err != nil {
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to persist healed rules to %s: %v", filePath, err)
			} else {
				logging.Kernel("Self-healing: persisted healed rules to %s", filePath)
			}
		}""",
"""		// Persist healed rules back to disk atomically if we have a file path
		if filePath != "" {
			tmpPath := filePath + ".tmp"
			if err := os.WriteFile(tmpPath, []byte(result.healedText), 0644); err != nil {
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to write temp file %s: %v", tmpPath, err)
			} else if err := os.Rename(tmpPath, filePath); err != nil {
				os.Remove(tmpPath) // cleanup
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to rename temp file to %s: %v", filePath, err)
			} else {
				logging.Kernel("Self-healing: persisted healed rules atomically to %s", filePath)
			}
		}""")

# 5. Infinite loop whitespace strip
content = content.replace(
"""		bodyLower := strings.ToLower(body)

		// === UBIQUITOUS PREDICATES ===""",
"""		bodyLower := strings.ToLower(body)
		bodyNoSpace := strings.ReplaceAll(bodyLower, " ", "")
		bodyNoSpace = strings.ReplaceAll(bodyNoSpace, "\\t", "")

		// === UBIQUITOUS PREDICATES ===""")

content = content.replace(
"""		for _, pred := range ubiquitousPredicates {
			if strings.Contains(body, pred) {
				// Single-predicate body with ubiquitous fact = infinite loop
				predCount := strings.Count(body, "(")""",
"""		for _, pred := range ubiquitousPredicates {
			if strings.Contains(bodyNoSpace, strings.ToLower(pred)) {
				// Single-predicate body with ubiquitous fact = infinite loop
				predCount := strings.Count(bodyNoSpace, "(")""")

with open(filepath, 'w', encoding='utf-8') as f:
    f.write(content)

print("Patch applied successfully")
