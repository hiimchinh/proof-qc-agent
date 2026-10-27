package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

type routeArguments struct {
	answer
	customerNoteNull bool
}

// decodeRoute checks tool structure; routing rules can supply a null customer note.
func decodeRoute(raw string) (routeArguments, error) {
	var ans routeArguments
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return ans, err
	}
	if err := requiredFields(fields, "decision", "reasons", "designer_note", "requested"); err != nil {
		return ans, err
	}
	if _, ok := fields["customer_note"]; !ok {
		return ans, fmt.Errorf("customer_note is required")
	}
	var requestedFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["requested"], &requestedFields); err != nil {
		return ans, err
	}
	if err := requiredFields(requestedFields, "shape", "material"); err != nil {
		return ans, err
	}
	for _, name := range []string{"width_in", "height_in"} {
		if _, ok := requestedFields[name]; !ok {
			return ans, fmt.Errorf("requested.%s is required", name)
		}
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans.answer); err != nil {
		return ans, err
	}
	ans.customerNoteNull = strings.TrimSpace(string(fields["customer_note"])) == "null"
	if !ans.Decision.valid() {
		return ans, fmt.Errorf("unknown decision %q", ans.Decision)
	}
	if s := ans.Requested.Shape; s != "none" && s != "die_cut" && s != "rectangle" {
		return ans, fmt.Errorf("unknown requested shape %q", s)
	}
	for _, v := range []*float64{ans.Requested.WidthIn, ans.Requested.HeightIn} {
		if v != nil && *v <= 0 {
			return ans, fmt.Errorf("requested size must be positive")
		}
	}
	return ans, nil
}

func requiredFields(fields map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		v, ok := fields[name]
		if !ok || string(v) == "null" {
			return fmt.Errorf("%s is required and must not be null", name)
		}
	}
	return nil
}
