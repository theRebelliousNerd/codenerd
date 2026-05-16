with open("tests/e2e/session_clean_loop_integration_test.go", "r") as f:
    lines = f.readlines()

new_lines = []
for i, line in enumerate(lines):
    if i in [54, 55, 56, 57]: # The bad lines
        continue
    new_lines.append(line)

with open("tests/e2e/session_clean_loop_integration_test.go", "w") as f:
    f.writelines(new_lines)
