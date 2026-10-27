package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"

	"golang.org/x/image/draw"

	"github.com/hiimchinh/proof-qc-agent/internal/artwork"
	"github.com/hiimchinh/proof-qc-agent/internal/notelint"
)

// The rules come from the proofing and support principles in the Sticker Mule Guide.
const systemPrompt = `You triage sticker artwork before a designer makes the customer's proof.

Choose one decision:
- ready_for_proof: a designer can make the proof now. Fixes a designer can do alone belong here and go in designer_note: remove a background, thicken thin lines, move art inside the safe margin, edit or add text the customer wrote out, resize art to the ordered size.
- recreate_artwork: the artwork is under 150 dpi at the ordered size. Always try to recreate low-resolution artwork; do not ask the customer for a better file.
- ask_customer: only the customer can unblock the proof. Use it when the comment refers to a file that is not attached, asks for a size, shape or material different from the order, gives instructions that contradict each other, or there is no artwork. These win over recreate_artwork.

Read the customer's comment thoroughly; it overrides what the image alone suggests.
The preflight measurements are exact; trust them over your view of the image.

designer_note: short imperative steps for the designer.
customer_note: what the customer reads. Use their first name. Be concise (under 60 words) and comprehensive. Never say sorry, unfortunately, can't, cannot, won't or unable; offer the solution instead. Avoid "appreciate". At most one exclamation point.`

var answerSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["decision", "reasons", "designer_note", "customer_note"],
  "properties": {
    "decision": {"type": "string", "enum": ["ready_for_proof", "recreate_artwork", "ask_customer"]},
    "reasons": {"type": "array", "items": {"type": "string"}},
    "designer_note": {"type": "string"},
    "customer_note": {"type": "string"}
  }
}`)

func userPrompt(o artwork.Order, r artwork.Report) (string, error) {
	order, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return "", err
	}
	pre, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Order:\n%s\n\nPreflight:\n%s\n\nThe image is attached, scaled down for viewing.", order, pre), nil
}

func repairPrompt(note string, vs []notelint.Violation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\nYour customer_note was: %q\nIt breaks these support principles:\n", note)
	for _, v := range vs {
		fmt.Fprintf(&b, "- %s (%s)\n", v.Rule, v.Detail)
	}
	b.WriteString("Answer again with the same decision and a customer_note that follows every rule.")
	return b.String()
}

// previewPNG scales the artwork to at most 768 px on its long side; resolution is
// judged by the preflight, so the model only needs to see the design.
func previewPNG(img image.Image) ([]byte, error) {
	const maxSide = 768
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if s := max(w, h); s > maxSide {
		w, h = w*maxSide/s, h*maxSide/s
	}
	dst := image.NewNRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return "there"
}
