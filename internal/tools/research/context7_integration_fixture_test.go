package research

import "testing"

// PinContext7MasterForIntegration is test-only: the external integration test
// can boot the production Cortex without creating a research -> system cycle.
func PinContext7MasterForIntegration(t *testing.T, body string) string {
	t.Helper()
	pinLocalGitHubFiles(t, map[string]string{
		"/fixtureowner/fixturerepo/master/llms.txt":      "[Introduction](https://github.com/fixtureowner/fixturerepo/blob/master/docs/intro.md)",
		"/fixtureowner/fixturerepo/master/docs/intro.md": body,
	})
	return rawFileURL(githubRepo("fixtureowner", "fixturerepo"), "master", "docs/intro.md")
}
