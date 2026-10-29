#!/usr/bin/env python3
"""
Malformed Input Generator for Stress Testing

Generates various malformed inputs for testing codeNERD's perception layer,
articulation parsing, and error handling.

Usage:
    python malformed_inputs.py [category] [--count N] [--output FILE]

Categories:
    perception   - Malformed NL inputs for transducer
    piggyback    - Malformed JSON for articulation
    mangle       - Invalid Mangle syntax
    all          - Generate all categories

Examples:
    python malformed_inputs.py perception --count 100
    python malformed_inputs.py all --output malformed_suite.json
"""

import argparse
import json
import random
import string
import sys
from typing import List, Dict, Any


def generate_perception_inputs(count: int) -> List[Dict[str, Any]]:
    """Generate malformed inputs for perception/transducer testing."""
    inputs = []

    # Empty and whitespace
    inputs.extend([
        {"name": "empty_string", "input": ""},
        {"name": "whitespace_only", "input": "   \t\n   "},
        {"name": "newlines_only", "input": "\n\n\n"},
        {"name": "tabs_only", "input": "\t\t\t"},
    ])

    # Unicode edge cases
    inputs.extend([
        {"name": "unicode_null", "input": "\x00"},
        {"name": "unicode_control", "input": "\x01\x02\x03\x04\x05"},
        {"name": "unicode_bom", "input": "\ufeff hello"},
        {"name": "unicode_rtl", "input": "\u202e reversed text"},
        {"name": "unicode_zero_width", "input": "hel\u200blo wor\u200cld"},
        {"name": "unicode_emoji_only", "input": "🔥💀🚀🤖👾"},
        {"name": "unicode_cjk", "input": "这是中文测试"},
        {"name": "unicode_arabic", "input": "هذا اختبار عربي"},
        {"name": "unicode_mixed", "input": "Hello 世界 مرحبا 🌍"},
    ])

    # Injection attempts
    inputs.extend([
        {"name": "sql_injection", "input": "'; DROP TABLE users; --"},
        {"name": "command_injection", "input": "$(rm -rf /)"},
        {"name": "command_injection_2", "input": "`cat /etc/passwd`"},
        {"name": "path_traversal", "input": "../../../../etc/passwd"},
        {"name": "xss_script", "input": "<script>alert('xss')</script>"},
        {"name": "xss_img", "input": "<img src=x onerror=alert(1)>"},
        {"name": "template_injection", "input": "{{7*7}}"},
        {"name": "ssti", "input": "${7*7}"},
    ])

    # Extreme lengths
    inputs.extend([
        {"name": "very_long", "input": "a" * 100000},
        {"name": "long_word", "input": "supercalifragilisticexpialidocious" * 1000},
        {"name": "many_words", "input": " ".join(["word"] * 10000)},
        {"name": "long_with_special", "input": "test " + ("@#$%^&*" * 5000)},
    ])

    # Ambiguous intents
    inputs.extend([
        {"name": "contradictory", "input": "don't review but also review the code"},
        {"name": "vague", "input": "do something maybe"},
        {"name": "multiple_intents", "input": "review test fix refactor deploy the code"},
        {"name": "negation", "input": "don't do anything"},
        {"name": "conditional", "input": "if it's Tuesday then review otherwise test"},
    ])

    # Special characters
    inputs.extend([
        {"name": "all_special", "input": "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
        {"name": "backslashes", "input": "\\\\\\\\\\\\\\\\"},
        {"name": "quotes", "input": "\"\"\"\"''''````"},
        {"name": "brackets", "input": "[[[[]]]]{{{{}}}}(((())))"},
        {"name": "regex_metachar", "input": ".*+?^${}()|[]\\"},
    ])

    # Format strings
    inputs.extend([
        {"name": "printf_format", "input": "%s%s%s%n%n%n"},
        {"name": "python_format", "input": "{0}{1}{2}"},
        {"name": "go_format", "input": "%v %+v %#v %T"},
    ])

    # Binary data (base64 encoded for JSON safety)
    inputs.extend([
        {"name": "binary_random", "input": "".join(chr(random.randint(0, 255)) for _ in range(100))},
        {"name": "null_bytes", "input": "hello\x00world\x00test"},
    ])

    # Fill remaining with random strings if needed
    while len(inputs) < count:
        length = random.randint(1, 1000)
        charset = string.printable + "".join(chr(i) for i in range(0x100, 0x1000))
        rand_input = "".join(random.choice(charset) for _ in range(length))
        inputs.append({"name": f"random_{len(inputs)}", "input": rand_input})

    return inputs[:count]


def generate_piggyback_inputs(count: int) -> List[Dict[str, Any]]:
    """Generate malformed Piggyback JSON for articulation testing."""
    inputs = []

    # Structural issues
    inputs.extend([
        {"name": "empty_object", "input": "{}"},
        {"name": "empty_array", "input": "[]"},
        {"name": "null_value", "input": "null"},
        {"name": "just_string", "input": '"just a string"'},
        {"name": "just_number", "input": "42"},
        {"name": "just_boolean", "input": "true"},
    ])

    # Missing fields
    inputs.extend([
        {"name": "missing_surface", "input": '{"control_packet": {"actions": []}}'},
        {"name": "missing_control", "input": '{"surface_response": "hello"}'},
        {"name": "missing_actions", "input": '{"surface_response": "hi", "control_packet": {}}'},
    ])

    # Wrong types
    inputs.extend([
        {"name": "surface_as_number", "input": '{"surface_response": 123, "control_packet": {"actions": []}}'},
        {"name": "surface_as_array", "input": '{"surface_response": [], "control_packet": {"actions": []}}'},
        {"name": "control_as_string", "input": '{"surface_response": "hi", "control_packet": "invalid"}'},
        {"name": "control_as_array", "input": '{"surface_response": "hi", "control_packet": []}'},
        {"name": "actions_as_string", "input": '{"surface_response": "hi", "control_packet": {"actions": "not array"}}'},
        {"name": "actions_as_object", "input": '{"surface_response": "hi", "control_packet": {"actions": {}}}'},
    ])

    # Truncated JSON
    inputs.extend([
        {"name": "truncated_1", "input": '{"surface_response": "hello", "control_packet": {"actions": ['},
        {"name": "truncated_2", "input": '{"surface_response": "hello", "control_packet": {'},
        {"name": "truncated_3", "input": '{"surface_response": "hello", "control_pack'},
        {"name": "truncated_4", "input": '{"surface_response": "hel'},
        {"name": "truncated_5", "input": '{"surface_res'},
    ])

    # Invalid JSON
    inputs.extend([
        {"name": "single_quotes", "input": "{'surface_response': 'hello'}"},
        {"name": "trailing_comma", "input": '{"surface_response": "hello",}'},
        {"name": "unquoted_key", "input": '{surface_response: "hello"}'},
        {"name": "invalid_escape", "input": '{"surface_response": "hello\\x"}'},
        {"name": "duplicate_keys", "input": '{"surface_response": "a", "surface_response": "b"}'},
    ])

    # Deeply nested
    deep_nest = '{"a":' * 100 + '{}' + '}' * 100
    inputs.append({"name": "deeply_nested", "input": deep_nest})

    # Large payloads
    inputs.extend([
        {"name": "huge_surface", "input": json.dumps({"surface_response": "x" * 1000000, "control_packet": {"actions": []}})},
        {"name": "many_actions", "input": json.dumps({"surface_response": "hi", "control_packet": {"actions": [{"type": "noop"}] * 10000}})},
    ])

    # Markdown wrapped
    inputs.extend([
        {"name": "markdown_json", "input": '```json\n{"surface_response": "hi", "control_packet": {"actions": []}}\n```'},
        {"name": "markdown_no_lang", "input": '```\n{"surface_response": "hi", "control_packet": {"actions": []}}\n```'},
    ])

    # Multiple objects
    inputs.extend([
        {"name": "multiple_objects", "input": '{"a": 1}{"b": 2}'},
        {"name": "array_of_objects", "input": '[{"surface_response": "a"}, {"surface_response": "b"}]'},
    ])

    return inputs[:count]


def generate_mangle_inputs(count: int) -> List[Dict[str, Any]]:
    """Generate malformed Mangle syntax for kernel testing."""
    inputs = []

    # Missing terminators
    inputs.extend([
        {"name": "missing_period", "input": "Decl foo(x: name)"},
        {"name": "missing_closing_paren", "input": "Decl foo(x: name."},
        {"name": "missing_colon", "input": "Decl foo(x name)."},
    ])

    # Invalid syntax
    inputs.extend([
        {"name": "invalid_predicate", "input": "Decl 123invalid(x: name)."},
        {"name": "invalid_variable", "input": "foo(123) :- bar(123)."},
        {"name": "invalid_type", "input": "Decl foo(x: invalidtype)."},
        {"name": "empty_predicate", "input": "Decl ()."},
        {"name": "empty_body", "input": "foo() :- ."},
    ])

    # Unsafe rules
    inputs.extend([
        {"name": "unbound_negation", "input": "blocked(X) :- not allowed(X)."},
        {"name": "unbound_head", "input": "foo(X, Y) :- bar(X)."},
        {"name": "aggregation_unbound", "input": "count(N) :- N = fn:count()."},
    ])

    # Recursive issues
    inputs.extend([
        {"name": "direct_recursion", "input": "foo(X) :- foo(X)."},
        {"name": "indirect_recursion", "input": "foo(X) :- bar(X).\nbar(X) :- foo(X)."},
        {"name": "infinite_derivation", "input": "next(N) :- next(M), N = M + 1."},
    ])

    # Special characters
    inputs.extend([
        {"name": "unicode_predicate", "input": "Decl 日本語(x: name)."},
        {"name": "special_in_name", "input": "Decl foo-bar(x: name)."},
        {"name": "sql_in_mangle", "input": "Decl foo(x: name); DROP TABLE users;."},
    ])

    return inputs[:count]


def generate_all_inputs(count_per_category: int) -> Dict[str, List[Dict[str, Any]]]:
    """Generate all categories of malformed inputs."""
    return {
        "perception": generate_perception_inputs(count_per_category),
        "piggyback": generate_piggyback_inputs(count_per_category),
        "mangle": generate_mangle_inputs(count_per_category),
    }


def main():
    parser = argparse.ArgumentParser(
        description='Generate malformed inputs for stress testing'
    )
    parser.add_argument(
        'category',
        nargs='?',
        default='all',
        choices=['perception', 'piggyback', 'mangle', 'all'],
        help='Category of inputs to generate (default: all)'
    )
    parser.add_argument(
        '--count', '-n',
        type=int,
        default=50,
        help='Number of inputs per category (default: 50)'
    )
    parser.add_argument(
        '--output', '-o',
        help='Output file (default: stdout)'
    )

    args = parser.parse_args()

    if args.category == 'all':
        result = generate_all_inputs(args.count)
    elif args.category == 'perception':
        result = {"perception": generate_perception_inputs(args.count)}
    elif args.category == 'piggyback':
        result = {"piggyback": generate_piggyback_inputs(args.count)}
    elif args.category == 'mangle':
        result = {"mangle": generate_mangle_inputs(args.count)}

    output = json.dumps(result, indent=2, ensure_ascii=False)

    if args.output:
        with open(args.output, 'w', encoding='utf-8') as f:
            f.write(output)
        print(f"Written to {args.output}")
    else:
        print(output)


if __name__ == '__main__':
    main()
