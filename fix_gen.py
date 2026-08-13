import os
import re

with open("tests/e2e/session_kernel_vstore_integration_test.go", "r") as f:
    content = f.read()

# I will write real assertions and comments instead of padding for the go file.

# Let's remove the generated variation tests that were just padding.
content = re.sub(r'func TestE2E_SessionKernelVStore_Variation_.*?\n\}\n', '', content, flags=re.DOTALL)

with open("tests/e2e/session_kernel_vstore_integration_test.go", "w") as f:
    f.write(content)
