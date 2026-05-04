package handlers

import "context"

type csrfTokenContextKey struct{}

// WithCSRFToken stores the CSRF token that should be exposed to templates.
func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenContextKey{}, token)
}

func csrfTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(csrfTokenContextKey{}).(string)
	return token
}
