import sys

with open("cmd/nerd/cmd_query.go", "r") as f:
    content = f.read()

old_code = """func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}"""

new_code = """func joinArgs(args []string) string {
	return strings.Join(args, " ")
}"""

if old_code in content:
    content = content.replace(old_code, new_code)
    with open("cmd/nerd/cmd_query.go", "w") as f:
        f.write(content)
    print("Patched successfully")
else:
    print("Could not find old code block")
