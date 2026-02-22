import re

with open('internal/campaign/decomposer.go', 'r') as f:
    content = f.read()

# Add package level variable
insertion_point = '// extractTopicsFromGoal tokenizes a goal into lowercase topics for Mangle selection.'
new_var = 'var goalTopicRegex = regexp.MustCompile(`[a-z0-9]+`)\n'
# Only add if not present
if new_var not in content:
    content = content.replace(insertion_point, new_var + insertion_point)

# Replace internal usage
# The regex must be exact.
old_usage = '	re := regexp.MustCompile(`[a-z0-9]+`)\n	matches := re.FindAllString(goal, -1)'
new_usage = '	matches := goalTopicRegex.FindAllString(goal, -1)'
content = content.replace(old_usage, new_usage)

with open('internal/campaign/decomposer.go', 'w') as f:
    f.write(content)
