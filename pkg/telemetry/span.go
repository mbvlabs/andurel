package telemetry

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Span is the handle returned by Start. End finishes it; add data with Set.
type Span struct {
	span trace.Span
}

// End finishes the span.
func (s Span) End() {
	if s.span != nil {
		s.span.End()
	}
}

func attrsFromArgs(args []any) []attribute.KeyValue {
	if len(args) == 0 {
		return nil
	}
	attrs := make([]attribute.KeyValue, 0, len(args)/2)
	for i := 0; i < len(args); i++ {
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprintf("arg_%d", i)
			attrs = append(attrs, attrValue(key, args[i]))
			continue
		}
		if i+1 >= len(args) {
			attrs = append(attrs, attribute.String(key, "!MISSING"))
			break
		}
		i++
		attrs = append(attrs, attrValue(key, args[i]))
	}
	return attrs
}

func attrValue(key string, value any) attribute.KeyValue {
	switch typed := value.(type) {
	case nil:
		return attribute.String(key, "<nil>")
	case string:
		return attribute.String(key, typed)
	case int:
		return attribute.Int(key, typed)
	case int64:
		return attribute.Int64(key, typed)
	case int32:
		return attribute.Int64(key, int64(typed))
	case float64:
		return attribute.Float64(key, typed)
	case bool:
		return attribute.Bool(key, typed)
	case time.Duration:
		return attribute.String(key, typed.String())
	case error:
		return attribute.String(key, typed.Error())
	case fmt.Stringer:
		return attribute.String(key, typed.String())
	default:
		return attribute.String(key, fmt.Sprint(typed))
	}
}

func errorFromArgs(args []any) error {
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key != "error" {
			continue
		}
		if err, ok := args[i+1].(error); ok {
			return err
		}
	}
	return nil
}
