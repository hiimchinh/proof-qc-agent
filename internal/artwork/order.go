// Package artwork runs the deterministic preflight a designer does before making a proof.
package artwork

// Product is the sticker shape the customer ordered.
type Product string

const (
	DieCut    Product = "die_cut"   // cut along the artwork outline
	Rectangle Product = "rectangle" // cut along the rectangle, background kept
)

// Order is the part of an order a proof depends on.
type Order struct {
	Customer string  `json:"customer"`
	Product  Product `json:"product"`
	WidthIn  float64 `json:"width_in"`
	HeightIn float64 `json:"height_in"`
	Comment  string  `json:"comment"`
}
