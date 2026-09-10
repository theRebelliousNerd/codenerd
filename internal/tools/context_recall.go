package tools

import "context"

type ContextRecall interface {
	Recall(context.Context, string, int, int) (string, error)
}
type contextRecallKey struct{}

func WithContextRecall(ctx context.Context, recall ContextRecall) context.Context {
	return context.WithValue(ctx, contextRecallKey{}, recall)
}
func ContextRecallFrom(ctx context.Context) ContextRecall {
	value, _ := ctx.Value(contextRecallKey{}).(ContextRecall)
	return value
}
