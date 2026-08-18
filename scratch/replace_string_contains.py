import os

def replace_in_file(filepath, old_str, new_str):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
    new_content = content.replace(old_str, new_str)
    with open(filepath, 'w', encoding='utf-8', newline='\n') as f:
        f.write(new_content)
    print(f"Replaced '{old_str}' with '{new_str}' in {filepath}")

def main():
    # 1. Update constitution.mg to use built-in :string:contains
    constitution_path = r"internal/core/defaults/policy/constitution.mg"
    replace_in_file(constitution_path, "string_contains(", ":string:contains(")

    # 2. Update schemas_safety.mg to comment out the string_contains declaration
    safety_schema_path = r"internal/core/defaults/schemas_safety.mg"
    with open(safety_schema_path, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    modified = False
    for i, line in enumerate(lines):
        if "Decl string_contains(" in line and not line.strip().startswith("#"):
            lines[i] = "# " + line
            modified = True
            print(f"Commented out string_contains declaration at line {i+1} in {safety_schema_path}")
            break
            
    if modified:
        with open(safety_schema_path, 'w', encoding='utf-8', newline='\n') as f:
            f.writelines(lines)

if __name__ == "__main__":
    main()
