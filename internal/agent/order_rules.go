package agent

import (
	"fmt"
	"math"
	"strings"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
)

// requested is what the customer's comment asks for, as extracted by the model.
// The model only reads; orderChanges decides whether that changes the order.
type requested struct {
	WidthIn  *float64 `json:"width_in"`
	HeightIn *float64 `json:"height_in"`
	Shape    string   `json:"shape"`
	Material string   `json:"material"`
}

// standardMaterials are what every order prints on unless it says otherwise.
var standardMaterials = []string{"", "vinyl", "white vinyl", "standard", "standard vinyl"}

// orderChanges lists every way the request differs from the order. Any change
// means the order itself must be edited before a proof makes sense.
func orderChanges(o artwork.Order, r requested) []string {
	var out []string
	same := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	switch {
	case r.WidthIn != nil && r.HeightIn != nil:
		w, h := *r.WidthIn, *r.HeightIn
		if !(same(w, o.WidthIn) && same(h, o.HeightIn)) && !(same(w, o.HeightIn) && same(h, o.WidthIn)) {
			out = append(out, fmt.Sprintf("a %gx%g in size instead of %gx%g in", w, h, o.WidthIn, o.HeightIn))
		}
	case r.WidthIn != nil || r.HeightIn != nil:
		// "Make it 5 inches" names one side; it is a change unless it matches either side.
		side := r.WidthIn
		if side == nil {
			side = r.HeightIn
		}
		if !same(*side, o.WidthIn) && !same(*side, o.HeightIn) {
			out = append(out, fmt.Sprintf("a %g in size instead of %gx%g in", *side, o.WidthIn, o.HeightIn))
		}
	}
	if s := r.Shape; s != "" && s != "none" && artwork.Product(s) != o.Product {
		out = append(out, fmt.Sprintf("a %s shape instead of %s", strings.ReplaceAll(s, "_", " "), strings.ReplaceAll(string(o.Product), "_", " ")))
	}
	if m := strings.ToLower(strings.TrimSpace(r.Material)); !contains(standardMaterials, m) {
		out = append(out, m+" instead of standard white vinyl")
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func orderChangeNote(customer string, changes []string) string {
	return fmt.Sprintf("Thanks %s. Your comment asks for %s, which changes your order. Reply to confirm and we'll update the order and send your proof.",
		firstName(customer), strings.Join(changes, " and "))
}
