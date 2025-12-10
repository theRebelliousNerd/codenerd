#!/usr/bin/env python3
"""
Generate Large Project for Stress Testing

Creates a synthetic Go project with configurable size for testing:
- World scanner with many files
- AST analysis with complex code
- Holographic context with large change sets
- Reviewer with many potential findings

Usage:
    python generate_large_project.py [output_dir] [--files N] [--depth D] [--complexity C]

Examples:
    python generate_large_project.py ./stress_project --files 100
    python generate_large_project.py ./large_project --files 1000 --depth 10
    python generate_large_project.py ./huge_project --files 10000 --depth 20 --complexity high
"""

import os
import sys
import argparse
import random
import string
from datetime import datetime

# Go code templates
GO_PACKAGE_TEMPLATE = '''package {package}

// {doc_comment}
// Generated: {timestamp}
// Stress test file {file_num} of {total_files}

import (
{imports}
)

{structs}

{functions}
'''

STRUCT_TEMPLATE = '''// {name} represents a {description}
type {name} struct {{
{fields}
}}
'''

FUNCTION_TEMPLATE = '''// {name} {description}
func {receiver}{name}({params}) {return_type} {{
{body}
}}
'''

# Complexity levels
COMPLEXITY_LEVELS = {
    'low': {'structs_per_file': 1, 'functions_per_file': 2, 'lines_per_function': 10},
    'medium': {'structs_per_file': 3, 'functions_per_file': 5, 'lines_per_function': 25},
    'high': {'structs_per_file': 5, 'functions_per_file': 10, 'lines_per_function': 50},
}

# Common imports
IMPORTS = [
    '"context"',
    '"errors"',
    '"fmt"',
    '"io"',
    '"log"',
    '"net/http"',
    '"os"',
    '"path/filepath"',
    '"strings"',
    '"sync"',
    '"time"',
]

# Field types
FIELD_TYPES = [
    'string', 'int', 'int64', 'float64', 'bool',
    'time.Time', 'context.Context', 'error',
    '[]byte', '[]string', 'map[string]interface{}',
    '*sync.Mutex', '*sync.RWMutex',
]

# Security issue patterns (for reviewer stress testing)
SECURITY_ISSUES = [
    'password := "{password}"  // Hardcoded credential',
    'apiKey := os.Getenv("API_KEY") // Sensitive env var',
    'exec.Command("sh", "-c", userInput)  // Command injection risk',
    'query := "SELECT * FROM users WHERE id = " + id  // SQL injection',
    'template.HTML(userContent)  // XSS vulnerability',
]


def random_name(prefix='', length=8):
    """Generate a random identifier name."""
    suffix = ''.join(random.choices(string.ascii_lowercase, k=length))
    return f"{prefix}{suffix.capitalize()}"


def generate_field():
    """Generate a random struct field."""
    name = random_name()
    typ = random.choice(FIELD_TYPES)
    tag = f'`json:"{name.lower()}"`'
    return f"\t{name} {typ} {tag}"


def generate_struct(name=None):
    """Generate a random struct."""
    if name is None:
        name = random_name('Struct')

    num_fields = random.randint(3, 10)
    fields = '\n'.join(generate_field() for _ in range(num_fields))

    return STRUCT_TEMPLATE.format(
        name=name,
        description=f"data structure for {name.lower()}",
        fields=fields
    )


def generate_function_body(lines, complexity='medium'):
    """Generate function body with specified complexity."""
    body_lines = []

    # Variable declarations
    body_lines.append('\tvar err error')
    body_lines.append('\t_ = err')

    # Conditional logic
    if random.random() > 0.5:
        body_lines.append('\tif ctx.Err() != nil {')
        body_lines.append('\t\treturn ctx.Err()')
        body_lines.append('\t}')

    # Loops
    if random.random() > 0.5 and lines > 10:
        body_lines.append('\tfor i := 0; i < 10; i++ {')
        body_lines.append(f'\t\t_ = i')
        body_lines.append('\t}')

    # Error handling
    body_lines.append('\tif err != nil {')
    body_lines.append('\t\treturn fmt.Errorf("operation failed: %w", err)')
    body_lines.append('\t}')

    # Fill remaining lines
    while len(body_lines) < lines:
        body_lines.append(f'\t// Processing step {len(body_lines)}')
        body_lines.append(f'\t_ = "{random_name()}"')

    body_lines.append('\treturn nil')
    return '\n'.join(body_lines[:lines])


def generate_function(receiver='', complexity='medium'):
    """Generate a random function."""
    name = random_name('Func')
    config = COMPLEXITY_LEVELS[complexity]

    params = 'ctx context.Context'
    return_type = 'error'
    body = generate_function_body(config['lines_per_function'], complexity)

    if receiver:
        receiver = f"({receiver[0].lower()} *{receiver}) "

    return FUNCTION_TEMPLATE.format(
        name=name,
        description=f"processes {name.lower()} operations",
        receiver=receiver,
        params=params,
        return_type=return_type,
        body=body
    )


def generate_go_file(package, file_num, total_files, complexity='medium'):
    """Generate a complete Go file."""
    config = COMPLEXITY_LEVELS[complexity]

    # Generate imports
    num_imports = random.randint(3, len(IMPORTS))
    selected_imports = random.sample(IMPORTS, num_imports)
    imports = '\n'.join(f'\t{imp}' for imp in selected_imports)

    # Generate structs
    structs = []
    struct_names = []
    for _ in range(config['structs_per_file']):
        name = random_name('Struct')
        struct_names.append(name)
        structs.append(generate_struct(name))

    # Generate functions
    functions = []
    for _ in range(config['functions_per_file']):
        receiver = random.choice([''] + struct_names) if struct_names else ''
        functions.append(generate_function(receiver, complexity))

    # Inject security issues occasionally (for reviewer testing)
    if random.random() > 0.9:
        issue = random.choice(SECURITY_ISSUES)
        functions.append(f'''
func {random_name('Issue')}() {{
    {issue}
}}
''')

    return GO_PACKAGE_TEMPLATE.format(
        package=package,
        doc_comment=f"Package {package} provides stress test functionality",
        timestamp=datetime.now().isoformat(),
        file_num=file_num,
        total_files=total_files,
        imports=imports,
        structs='\n'.join(structs),
        functions='\n'.join(functions)
    )


def generate_project(output_dir, num_files, max_depth, complexity):
    """Generate the complete project structure."""
    os.makedirs(output_dir, exist_ok=True)

    # Create go.mod
    with open(os.path.join(output_dir, 'go.mod'), 'w') as f:
        f.write(f'''module stress_project

go 1.21
''')

    # Create README
    with open(os.path.join(output_dir, 'README.md'), 'w') as f:
        f.write(f'''# Stress Test Project

Generated: {datetime.now().isoformat()}
Files: {num_files}
Max Depth: {max_depth}
Complexity: {complexity}

This project is generated for codeNERD stress testing.
''')

    # Generate package directories
    packages = ['main', 'internal', 'pkg', 'cmd', 'api', 'models', 'services', 'utils']
    files_per_package = num_files // len(packages)

    file_num = 0
    for pkg in packages:
        pkg_dir = os.path.join(output_dir, pkg)

        # Create nested structure based on depth
        for depth in range(random.randint(1, max_depth)):
            subdir = os.path.join(pkg_dir, *[random_name('sub') for _ in range(depth)])
            os.makedirs(subdir, exist_ok=True)

            # Create files in this directory
            for _ in range(files_per_package // max_depth):
                file_num += 1
                if file_num > num_files:
                    break

                filename = f"{random_name('file')}.go"
                filepath = os.path.join(subdir, filename)

                pkg_name = os.path.basename(subdir)
                content = generate_go_file(pkg_name, file_num, num_files, complexity)

                with open(filepath, 'w') as f:
                    f.write(content)

                if file_num % 100 == 0:
                    print(f"Generated {file_num}/{num_files} files...")

    # Create main.go
    main_path = os.path.join(output_dir, 'main.go')
    with open(main_path, 'w') as f:
        f.write('''package main

import "fmt"

func main() {
    fmt.Println("Stress test project")
}
''')

    print(f"\nGenerated {file_num} files in {output_dir}")
    return file_num


def main():
    parser = argparse.ArgumentParser(
        description='Generate a large Go project for stress testing'
    )
    parser.add_argument(
        'output_dir',
        nargs='?',
        default='./stress_project',
        help='Output directory (default: ./stress_project)'
    )
    parser.add_argument(
        '--files', '-f',
        type=int,
        default=100,
        help='Number of files to generate (default: 100)'
    )
    parser.add_argument(
        '--depth', '-d',
        type=int,
        default=5,
        help='Maximum directory depth (default: 5)'
    )
    parser.add_argument(
        '--complexity', '-c',
        choices=['low', 'medium', 'high'],
        default='medium',
        help='Code complexity level (default: medium)'
    )

    args = parser.parse_args()

    print(f"Generating stress test project:")
    print(f"  Output: {args.output_dir}")
    print(f"  Files: {args.files}")
    print(f"  Depth: {args.depth}")
    print(f"  Complexity: {args.complexity}")
    print()

    generate_project(
        args.output_dir,
        args.files,
        args.depth,
        args.complexity
    )


if __name__ == '__main__':
    main()
