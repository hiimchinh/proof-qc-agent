package artwork

import "testing"

func TestOrderValidate(t *testing.T) {
	ok := Order{Customer: "Brian", Product: DieCut, WidthIn: 3, HeightIn: 3}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid order: %v", err)
	}
	bad := []Order{
		{Customer: "Brian", Product: "diecut", WidthIn: 3, HeightIn: 3},
		{Customer: "Brian", Product: DieCut, WidthIn: 0, HeightIn: 3},
		{Customer: "Brian", Product: Rectangle, WidthIn: 3, HeightIn: -1},
	}
	for _, o := range bad {
		if err := o.Validate(); err == nil {
			t.Errorf("%+v: want an error", o)
		}
	}
}
