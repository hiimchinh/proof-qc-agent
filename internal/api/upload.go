package api

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg" // uploads may be JPEG
	_ "image/png"
	"strings"
)

const (
	maxUploadBytes = 4 << 20
	maxUploadSide  = 6000
)

// decodeUpload turns base64 into an image, checking size before each step
// that costs memory: encoded length, decoded bytes, then pixel dimensions from
// the header, so a small file that inflates to a huge bitmap is never decoded.
func decodeUpload(b64 string) (image.Image, error) {
	s := strings.TrimSpace(b64)
	if strings.HasPrefix(s, "data:") {
		if _, rest, ok := strings.Cut(s, ","); ok {
			s = rest
		}
	}
	if base64.StdEncoding.DecodedLen(len(s)) > maxUploadBytes+2 {
		return nil, badInput("the file is over the 4 MB limit")
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, badInput("imageBase64 is not valid base64")
	}
	if len(raw) > maxUploadBytes {
		return nil, badInput("the file is over the 4 MB limit")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, badInput("the file must be a PNG or JPEG")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxUploadSide || cfg.Height > maxUploadSide {
		return nil, badInput("the image is %dx%d px; each side must be 1 to 6000 px", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, badInput("the %s file could not be decoded: %v", strings.ToUpper(format), err)
	}
	return img, nil
}
