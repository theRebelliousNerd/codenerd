import re

with open("cmd/nerd/chat/process.go", "r") as f:
    content = f.read()

# Fix the context leak
# Look for line 1050
lines = content.split('\n')
for i, line in enumerate(lines):
    if 'context.WithTimeout' in line and i > 1040 and i < 1060:
        # Check if it lacks a defer cancel()
        if 'defer cancel()' not in lines[i+1]:
            lines.insert(i+1, '\t\tdefer cancel()')
            break

with open("cmd/nerd/chat/process.go", "w") as f:
    f.write('\n'.join(lines))
