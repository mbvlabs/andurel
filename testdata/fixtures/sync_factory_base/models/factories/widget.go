package factories

import (
	"math"
	"strings"

	"example.com/app/models"
	"github.com/go-faker/faker/v4"
)

// Hand-edited / stale factory for sync goldens.
// Sync must regenerate Andurel-owned decls (including WithWidgetName) and
// preserve CustomWidgetScore. The strings import is only used by the stale
// name override and should be dropped after sync.

type WidgetFactory struct {
	models.Widget
}

type WidgetOption func(*WidgetFactory)

func BuildWidget(opts ...WidgetOption) models.Widget {
	f := &WidgetFactory{
		Widget: models.Widget{
			Name: faker.Name(),
			// Quantity/Active intentionally omitted so the generated region is stale.
		},
	}

	for _, opt := range opts {
		opt(f)
	}

	return f.Widget
}

// WithWidgetName is a same-name override of the generated option; sync replaces it.
func WithWidgetName(value string) WidgetOption {
	return func(f *WidgetFactory) {
		f.Widget.Name = strings.ToUpper(value)
	}
}

// CustomWidgetScore is a hand-written helper outside generated regions.
func CustomWidgetScore() int {
	return int(math.Max(1, 2))
}
