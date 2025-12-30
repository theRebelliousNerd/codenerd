#!/bin/bash
# Verification script for Mangle Adversarial Test Suite
# Counts test patterns and validates structure

echo "=== Mangle Adversarial Test Suite Verification ==="
echo ""

# Count files by category
echo "Files by Category:"
for dir in syntactic safety types loops structures; do
    count=$(find "$dir" -name "*.mg" 2>/dev/null | wc -l)
    echo "  $dir: $count files"
done

echo ""
echo "Total .mg files: $(find . -name "*.mg" | wc -l)"

echo ""
echo "Test Pattern Counts (approximate):"

# Count ERROR comments (each marks a test pattern)
echo "  Syntactic errors: $(grep -h "# ERROR:" syntactic/*.mg 2>/dev/null | wc -l)"
echo "  Safety errors: $(grep -h "# ERROR:" safety/*.mg 2>/dev/null | wc -l)"
echo "  Type errors: $(grep -h "# ERROR:" types/*.mg 2>/dev/null | wc -l)"
echo "  Loop errors: $(grep -h "# ERROR:" loops/*.mg 2>/dev/null | wc -l)"
echo "  Structure errors: $(grep -h "# ERROR:" structures/*.mg 2>/dev/null | wc -l)"

total_errors=$(grep -rh "# ERROR:" . 2>/dev/null | wc -l)
echo "  Total error patterns: $total_errors"

echo ""
echo "Correct examples: $(grep -rh "# CORRECT:" . 2>/dev/null | wc -l)"

echo ""
echo "Documentation:"
[ -f README.md ] && echo "  ✓ README.md exists" || echo "  ✗ README.md missing"
[ -f INDEX.md ] && echo "  ✓ INDEX.md exists" || echo "  ✗ INDEX.md missing"

echo ""
echo "=== Verification Complete ==="
