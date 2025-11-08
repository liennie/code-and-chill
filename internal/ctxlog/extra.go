package ctxlog

import (
	"context"
)

type extra struct {
	val []any
}

type extraCtxKey struct{}

var extraKey extraCtxKey

func WithExtra(ctx context.Context) context.Context {
	return context.WithValue(ctx, extraKey, &extra{val: []any{}})
}

func AddExtra(ctx context.Context, keyvals ...any) {
	extraVal, ok := ctx.Value(extraKey).(*extra)
	if !ok {
		return
	}
	extraVal.val = append(extraVal.val, keyvals...)
}

func GetExtra(ctx context.Context) []any {
	extraVal, ok := ctx.Value(extraKey).(*extra)
	if !ok {
		return nil
	}
	return extraVal.val
}
