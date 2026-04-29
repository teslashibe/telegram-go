package telegram

import (
	"fmt"

	"go.uber.org/zap"
)

// logFields turns a struct (params) into a small set of zap fields for
// observability. Keeps secrets out of logs by stringifying via %+v which
// only includes exported fields we control.
func logFields(v any) []zap.Field {
	return []zap.Field{zap.String("params", fmt.Sprintf("%+v", v))}
}
