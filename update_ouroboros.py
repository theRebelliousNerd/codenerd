import sys

def replace_wrapper_logic(content):
    search_str = """		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			var toolInput ToolInput
			if err := json.Unmarshal(scanner.Bytes(), &toolInput); err == nil {
				input = toolInput.Input
			} else {
				input = strings.TrimSpace(scanner.Text())
			}
		}"""

    replace_str = """		// Read up to 10MB to avoid OOM
		reader := io.LimitReader(os.Stdin, 10*1024*1024)
		inputBytes, err := io.ReadAll(reader)
		if err == nil && len(inputBytes) > 0 {
			var toolInput ToolInput
			if err := json.Unmarshal(inputBytes, &toolInput); err == nil {
				input = toolInput.Input
			} else {
				input = strings.TrimSpace(string(inputBytes))
			}
		}"""

    if search_str in content:
        content = content.replace(search_str, replace_str)
        print("Replaced scanner logic.")
    else:
        print("Scanner logic not found.")
        # Try finding with "io" instead of "bufio" if my sed worked
        search_str_io = search_str.replace("bufio", "io")
        if search_str_io in content:
             content = content.replace(search_str_io, replace_str)
             print("Replaced scanner logic (sed altered).")
        else:
             print("Could not find scanner logic block.")
             # Debug: print what we are looking for vs what might be there
             # But better not print huge content.

    # Also check if import "bufio" needs replacement
    # It might have been replaced by sed already.
    # We want to replace "bufio" with "io" in the import block of generated code.

    # Locate import block in generated code string
    import_block_start = 'import ('
    # We look for "bufio" inside the string literal

    if '"bufio"' in content:
         content = content.replace('"bufio"', '"io"')
         print("Replaced bufio import with io.")
    elif '"io"' in content:
         print("io import already present (or sed did it).")

    return content

if __name__ == "__main__":
    file_path = "internal/autopoiesis/ouroboros.go"
    try:
        with open(file_path, "r") as f:
            content = f.read()

        new_content = replace_wrapper_logic(content)

        if content != new_content:
            with open(file_path, "w") as f:
                f.write(new_content)
            print("File updated successfully.")
        else:
            print("No changes made.")

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
