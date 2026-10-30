package inertia

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

type FromStructBase struct {
	Title  string `json:"title"`
	Secret string `json:"-"`
}

type FromStructOuter struct {
	FromStructBase
	X string `json:"x"`
}

type FromStructTagged struct {
	FromStructBase `json:"base"`
	X              string `json:"x"`
}

type FromStructPointer struct {
	*FromStructBase
	X string `json:"x"`
}

type FromStructShadow struct {
	FromStructBase
	Title int `json:"title"`
}

type FromStructLeft struct {
	LeftValue string `json:"value"`
}

type FromStructRight struct {
	RightValue string `json:"value"`
}

type FromStructLazy struct {
	Load func(context.Context) (any, error) `json:"load"`
}

func TestFromStructFlattensUntaggedEmbeddedFieldsLikeJSON(t *testing.T) {
	props, err := FromStructChecked(FromStructOuter{
		Title: "the title", Secret: "hidden",
		X: "outer",
	})
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}

	encoded, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal props: %v", err)
	}
	if string(encoded) != `{"title":"the title","x":"outer"}` {
		t.Fatalf("props JSON = %s, want flattened title and x", encoded)
	}
	if _, ok := props["FromStructBase"]; ok {
		t.Fatalf("untagged embed stayed nested: %#v", props)
	}
}

func TestFromStructKeepsTaggedEmbeddedFieldsNested(t *testing.T) {
	props, err := FromStructChecked(FromStructTagged{
		Title: "the title",
		X:     "outer",
	})
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}

	encoded, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal props: %v", err)
	}
	if string(encoded) != `{"base":{"title":"the title"},"x":"outer"}` {
		t.Fatalf("props JSON = %s, want a nested base object", encoded)
	}
}

func TestFromStructFlattensOnlyNonNilPointerEmbeds(t *testing.T) {
	nilProps, err := FromStructChecked(FromStructPointer{X: "outer"})
	if err != nil {
		t.Fatalf("FromStructChecked(nil): %v", err)
	}
	encoded, err := json.Marshal(nilProps)
	if err != nil {
		t.Fatalf("marshal nil props: %v", err)
	}
	if string(encoded) != `{"x":"outer"}` {
		t.Fatalf("nil pointer props JSON = %s, want x only", encoded)
	}

	nonNilProps, err := FromStructChecked(FromStructPointer{
		FromStructBase: &FromStructBase{Title: "the title"},
		X:              "outer",
	})
	if err != nil {
		t.Fatalf("FromStructChecked(non-nil): %v", err)
	}
	encoded, err = json.Marshal(nonNilProps)
	if err != nil {
		t.Fatalf("marshal non-nil props: %v", err)
	}
	if string(encoded) != `{"title":"the title","x":"outer"}` {
		t.Fatalf("non-nil pointer props JSON = %s, want flattened title", encoded)
	}
}

func TestFromStructUsesOuterFieldOverEmbeddedConflict(t *testing.T) {
	props, err := FromStructChecked(FromStructShadow{
		FromStructBase: FromStructBase{Title: "embedded"},
		Title:          7,
	})
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}

	encoded, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal props: %v", err)
	}
	if string(encoded) != `{"title":7}` {
		t.Fatalf("props JSON = %s, want the outer title field", encoded)
	}
}

func TestFromStructOmitsAmbiguousSameDepthEmbeddedConflict(t *testing.T) {
	left := reflect.TypeFor[FromStructLeft]()
	right := reflect.TypeFor[FromStructRight]()
	ambiguous := reflect.StructOf([]reflect.StructField{
		{Name: "FromStructLeft", Type: left, Anonymous: true},
		{Name: "FromStructRight", Type: right, Anonymous: true},
		{Name: "X", Type: reflect.TypeFor[string](), Tag: `json:"x"`},
	})
	value := reflect.New(ambiguous).Elem()
	value.Field(0).Set(reflect.ValueOf(FromStructLeft{LeftValue: "left"}))
	value.Field(1).Set(reflect.ValueOf(FromStructRight{RightValue: "right"}))
	value.Field(2).SetString("outer")

	props, err := FromStructChecked(value.Interface())
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}

	encoded, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal props: %v", err)
	}
	if string(encoded) != `{"x":"outer"}` {
		t.Fatalf("props JSON = %s, want the ambiguous value omitted", encoded)
	}
}

func TestFromStructPreservesLazyPropValues(t *testing.T) {
	load := func(context.Context) (any, error) { return "loaded", nil }
	props, err := FromStructChecked(FromStructLazy{Load: load})
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}
	got, ok := props["load"].(func(context.Context) (any, error))
	if !ok {
		t.Fatalf("load prop = %#v, want the lazy function preserved", props["load"])
	}
	value, err := got(context.Background())
	if err != nil || value != "loaded" {
		t.Fatalf("lazy prop returned %v, %v", value, err)
	}
}

func TestFromStructPreservesPropValues(t *testing.T) {
	lazy := Deferred(func(context.Context) (any, error) { return "loaded", nil })
	props, err := FromStructChecked(struct {
		Value Prop `json:"value"`
	}{Value: lazy})
	if err != nil {
		t.Fatalf("FromStructChecked: %v", err)
	}
	prop, ok := props["value"].(Prop)
	if !ok {
		t.Fatalf("value prop = %#v, want the lazy Prop preserved", props["value"])
	}
	if !prop.deferred || !prop.optional {
		t.Fatalf("prop = %#v, want deferred and optional policy preserved", prop)
	}
}
