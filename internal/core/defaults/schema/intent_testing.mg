# Intent Definitions - Testing
# SECTION 11: TESTING (/test) - TESTER SHARD
# Test-related requests including test generation, execution, and coverage.

intent_definition("Run the tests.", /test, "all").
intent_category("Run the tests.", /mutation).

intent_definition("Run tests.", /test, "all").
intent_category("Run tests.", /mutation).

intent_definition("Run all tests.", /test, "all").
intent_category("Run all tests.", /mutation).

intent_definition("Test this.", /test, "context_file").
intent_category("Test this.", /mutation).

intent_definition("Test this file.", /test, "context_file").
intent_category("Test this file.", /mutation).

intent_definition("Test this function.", /test, "function").
intent_category("Test this function.", /mutation).

intent_definition("Generate unit tests.", /test, "unit").
intent_category("Generate unit tests.", /mutation).

intent_definition("Generate unit tests for this.", /test, "context_file").
intent_category("Generate unit tests for this.", /mutation).

intent_definition("Write tests for this.", /test, "context_file").
intent_category("Write tests for this.", /mutation).

intent_definition("Write tests for this function.", /test, "focused_symbol").
intent_category("Write tests for this function.", /mutation).

intent_definition("Add test coverage.", /test, "coverage").
intent_category("Add test coverage.", /mutation).

intent_definition("Create integration tests.", /test, "integration").
intent_category("Create integration tests.", /mutation).

intent_definition("Test this code.", /test, "context_file").
intent_category("Test this code.", /mutation).

intent_definition("Run go test.", /test, "go_test").
intent_category("Run go test.", /mutation).

intent_definition("Run the unit tests.", /test, "unit").
intent_category("Run the unit tests.", /mutation).

intent_definition("Run integration tests.", /test, "integration").
intent_category("Run integration tests.", /mutation).

intent_definition("Check test coverage.", /test, "coverage").
intent_category("Check test coverage.", /query).

intent_definition("What's the coverage?", /test, "coverage").
intent_category("What's the coverage?", /query).

intent_definition("Generate a test file.", /test, "test_file").
intent_category("Generate a test file.", /mutation).

intent_definition("Create a test for this.", /test, "context_file").
intent_category("Create a test for this.", /mutation).

intent_definition("Add a test case.", /test, "test_case").
intent_category("Add a test case.", /mutation).

intent_definition("Write a table-driven test.", /test, "table_test").
intent_category("Write a table-driven test.", /mutation).

intent_definition("Generate a mock.", /test, "mock").
intent_category("Generate a mock.", /mutation).

intent_definition("Create a mock for this.", /test, "mock").
intent_category("Create a mock for this.", /mutation).

intent_definition("Add test fixtures.", /test, "fixtures").
intent_category("Add test fixtures.", /mutation).

intent_definition("Run benchmarks.", /test, "benchmark").
intent_category("Run benchmarks.", /mutation).

intent_definition("Write a benchmark.", /test, "benchmark").
intent_category("Write a benchmark.", /mutation).

intent_definition("TDD this.", /test, "tdd").
intent_category("TDD this.", /mutation).

intent_definition("Test-driven development.", /test, "tdd").
intent_category("Test-driven development.", /mutation).
