with open("tests/e2e/session_clean_loop_integration_test.go", "r") as f:
    content = f.read()
content = content.replace('res, err := exec.Process(ctx, "Do it")', 'res, err := exec.Process(ctx, "Do it")\n\t_ = res')
content = content.replace('res, err := exec.Process(ctx, "")', 'res, err := exec.Process(ctx, "")\n\t_ = res')
content = content.replace('res, err := exec.Process(ctx, massiveInput)', 'res, err := exec.Process(ctx, massiveInput)\n\t_ = res')
with open("tests/e2e/session_clean_loop_integration_test.go", "w") as f:
    f.write(content)
