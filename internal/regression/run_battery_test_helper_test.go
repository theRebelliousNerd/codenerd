package regression

import "context"

// RunBattery executes all tasks in order using the local shell, returning only
// the per-task results. Production runs batteries through RunBatteryWithOptions
// (`nerd regression run`); this results-only spelling is the tests'.
func RunBattery(ctx context.Context, b *Battery, workdir string) ([]Result, error) {
	summary, err := RunBatteryWithOptions(ctx, b, RunOptions{Workdir: workdir})
	if err != nil {
		return nil, err
	}
	if summary.Total == 0 {
		return nil, nil
	}
	return summary.Results, nil
}
