package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func pngBase64(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := h / 4; y < 3*h/4; y++ {
		for x := w / 4; x < 3*w/4; x++ {
			img.Set(x, y, color.NRGBA{200, 30, 40, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func uploadInput(image string) map[string]any {
	in := sampleInput()
	delete(in, "sampleId")
	in["imageBase64"] = image
	return in
}

// tinyInput is a 30 px upload at 300 dpi, for tests that make many calls and
// care about quotas, not artwork.
func tinyInput(t *testing.T) map[string]any {
	in := uploadInput(pngBase64(t, 30, 30))
	in["widthIn"], in["heightIn"] = 0.1, 0.1
	return in
}

// rejects posts the input and wants a BAD_INPUT error naming want, with no model call.
func rejects(t *testing.T, in map[string]any, want string) {
	t.Helper()
	s := newTestServer(t, 3)
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in})
	if resp.code() != "BAD_INPUT" || !strings.Contains(resp.Errors[0].Message, want) {
		t.Fatalf("errors = %+v, want BAD_INPUT mentioning %q", resp.Errors, want)
	}
	if n := s.llm.callCount(); n != 0 {
		t.Fatalf("model called %d times for a rejected request", n)
	}
}

func TestUploadedPNGIsReviewed(t *testing.T) {
	s := newTestServer(t, 3)
	in := uploadInput(pngBase64(t, 600, 600))
	in["widthIn"], in["heightIn"], in["model"] = 2, 2, "x-ai/grok-4.7"
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in})
	if len(resp.Errors) > 0 {
		t.Fatalf("errors: %+v", resp.Errors)
	}
	if !strings.Contains(string(resp.Data), `"effectiveDpi":300`) || s.llm.models[0] != "x-ai/grok-4.7" {
		t.Fatalf("data = %s, models = %v", resp.Data, s.llm.models)
	}
}

func TestRejectsModelsOutsideTheAllowList(t *testing.T) {
	in := sampleInput()
	in["model"] = "openai/gpt-6.1-pro"
	rejects(t, in, "model")
}

func TestRejectsInvalidOrders(t *testing.T) {
	in := sampleInput()
	in["widthIn"] = 0
	rejects(t, in, "both sides must be positive")
}

func TestRejectsUnknownSamples(t *testing.T) {
	in := sampleInput()
	in["sampleId"] = "../cases.json"
	rejects(t, in, "sample")
}

func TestRejectsBothOrNoArtwork(t *testing.T) {
	in := sampleInput()
	in["imageBase64"] = pngBase64(t, 10, 10)
	rejects(t, in, "either imageBase64 or sampleId")

	in = sampleInput()
	delete(in, "sampleId")
	rejects(t, in, "either imageBase64 or sampleId")
}

func TestRejectsFilesOver4MB(t *testing.T) {
	raw := make([]byte, 4<<20+1)
	_, _ = rand.Read(raw)
	rejects(t, uploadInput(base64.StdEncoding.EncodeToString(raw)), "4 MB")
}

func TestRejectsImagesOver6000PxASide(t *testing.T) {
	rejects(t, uploadInput(pngBase64(t, 6001, 8)), "6000 px")
}

func TestRejectsFilesThatAreNotPNGOrJPEG(t *testing.T) {
	rejects(t, uploadInput(base64.StdEncoding.EncodeToString([]byte("GIF89a not really"))), "PNG or JPEG")
}

func TestRejectsLongComments(t *testing.T) {
	in := sampleInput()
	in["comment"] = strings.Repeat("a", 1001)
	rejects(t, in, "comment")
}

func TestDailyCapStopsCallsBeforeTheModel(t *testing.T) {
	s, in := newTestServer(t, 0.0005), tinyInput(t)
	if resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in}); len(resp.Errors) > 0 {
		t.Fatalf("first review under the cap: %+v", resp.Errors)
	}
	// 0.0004569 spent, under 0.0005: one more call may start.
	if resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in}); len(resp.Errors) > 0 {
		t.Fatalf("second review under the cap: %+v", resp.Errors)
	}
	resp := s.do(t, "198.51.100.2", reviewMutation, map[string]any{"in": in})
	if resp.code() != "DAILY_CAP" || s.llm.callCount() != 2 {
		t.Fatalf("errors = %+v after %d calls, want DAILY_CAP and no third call", resp.Errors, s.llm.callCount())
	}
}

func TestZeroCapBlocksEveryCall(t *testing.T) {
	s, in := newTestServer(t, 0), tinyInput(t)
	resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in})
	if resp.code() != "DAILY_CAP" || s.llm.callCount() != 0 {
		t.Fatalf("errors = %+v, calls = %d; want DAILY_CAP before any call", resp.Errors, s.llm.callCount())
	}
}

func TestRateLimitIsPerClientIP(t *testing.T) {
	s, in := newTestServer(t, 3), tinyInput(t)
	for i := range 20 {
		if resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in}); len(resp.Errors) > 0 {
			t.Fatalf("review %d: %+v", i+1, resp.Errors)
		}
	}
	if resp := s.do(t, "198.51.100.1", reviewMutation, map[string]any{"in": in}); resp.code() != "RATE_LIMITED" {
		t.Fatalf("21st review: errors = %+v, want RATE_LIMITED", resp.Errors)
	}
	if resp := s.do(t, "198.51.100.2", reviewMutation, map[string]any{"in": in}); len(resp.Errors) > 0 {
		t.Fatalf("another client: %+v", resp.Errors)
	}
}

func TestRecentReviewsLimitIsBounded(t *testing.T) {
	s := newTestServer(t, 3)
	resp := s.do(t, "198.51.100.1", `{ recentReviews(limit: 51) { model } }`, nil)
	if resp.code() != "BAD_INPUT" {
		t.Fatalf("errors = %+v, want BAD_INPUT", resp.Errors)
	}
}
