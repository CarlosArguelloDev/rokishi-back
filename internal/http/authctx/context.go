package authctx

import (
	"context"

	"rokishi-back/internal/models"
)

type userContextKey struct{}

func WithUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func User(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(models.User)
	return user, ok
}
