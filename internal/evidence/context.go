package evidence

import "context"

type contractKey struct{}

func WithContract(ctx context.Context, c Contract) context.Context {
	c.Obligations = append([]Obligation(nil), c.Obligations...)
	return context.WithValue(ctx, contractKey{}, c)
}

func ContractFromContext(ctx context.Context) (Contract, bool) {
	c, ok := ctx.Value(contractKey{}).(Contract)
	c.Obligations = append([]Obligation(nil), c.Obligations...)
	return c, ok
}
