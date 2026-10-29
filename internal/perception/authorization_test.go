package perception

import (
	"context"
	"slices"
	"testing"
)

func TestAuthorization_ZeroIsNothingAndMergeOnlyAdds(t *testing.T) {
	var zero Authorization
	if !zero.IsZero() || zero.Has(AuthorizeCommit) || len(zero.Kinds()) != 0 {
		t.Fatal("the zero authorization allows something")
	}
	commit := AuthorizationOf(AuthorizeCommit)
	if !commit.Has(AuthorizeCommit) || commit.IsZero() {
		t.Fatal("commit authorization lost its kind")
	}
	if got := zero.Merge(commit); !got.Has(AuthorizeCommit) {
		t.Fatal("merging into zero dropped the kind")
	}
	if got := commit.Merge(zero); !got.Has(AuthorizeCommit) {
		t.Fatal("merging zero dropped the kind")
	}
	if got := AuthorizationOf("", AuthorizeCommit, AuthorizeCommit).Kinds(); !slices.Equal(got, []AuthorizationKind{AuthorizeCommit}) {
		t.Fatalf("kinds = %v, want one commit", got)
	}
	if !AuthorizationOf().IsZero() {
		t.Fatal("no kinds is not zero")
	}
}

func TestAuthorization_KindsAreACopy(t *testing.T) {
	a := AuthorizationOf(AuthorizeCommit)
	a.Kinds()[0] = "everything"
	if !a.Has(AuthorizeCommit) || a.Has("everything") {
		t.Fatal("a caller rewrote the authorization through Kinds()")
	}
}

// A derived context holds what its parent held and can only add: a task never
// loses the request's authorization on the way down, and a context that has
// none gains none by being asked.
func TestAuthorization_ContextInheritsAndNeverLoses(t *testing.T) {
	if !AuthorizationFromContext(context.Background()).IsZero() || !AuthorizationFromContext(nil).IsZero() {
		t.Fatal("a bare context carries an authorization")
	}
	parent := WithAuthorization(context.Background(), AuthorizationOf(AuthorizeCommit))
	child := WithAuthorization(parent, Authorization{})
	if !AuthorizationFromContext(child).Has(AuthorizeCommit) {
		t.Fatal("a zero authorization on a derived context dropped its parent's")
	}
	if !AuthorizationFromContext(context.WithValue(parent, struct{}{}, 1)).Has(AuthorizeCommit) {
		t.Fatal("an unrelated derived context dropped the authorization")
	}
	if got := WithAuthorization(nil, AuthorizationOf(AuthorizeCommit)); !AuthorizationFromContext(got).Has(AuthorizeCommit) {
		t.Fatal("a nil parent lost the authorization")
	}
}
