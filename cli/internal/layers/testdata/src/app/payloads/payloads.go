package payloads

import (
	"encoding/json"
	"time"
)

type WidgetProps struct {
	Title    string          `json:"title"`
	Created  time.Time       `json:"created"`
	Metadata json.RawMessage `json:"metadata"`
}
