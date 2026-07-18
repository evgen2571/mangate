package dataset

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ValidatedImage struct {
	MIMEType, SHA256, PerceptualHash string
	Width, Height                    int
	Bytes                            int64
}

func ValidateFile(path string, cfg Validation) (ValidatedImage, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return ValidatedImage{}, "missing_file", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ValidatedImage{}, "missing_file", err
	}
	if info.Size() == 0 {
		return ValidatedImage{}, "empty_file", fmt.Errorf("image file is empty")
	}
	decoded, format, err := image.DecodeConfig(file)
	if err != nil {
		return ValidatedImage{}, "decode_config_failed", err
	}
	mimeType := mimeFor(format, filepath.Ext(path))
	if mimeType == "" {
		return ValidatedImage{}, "unsupported_format", fmt.Errorf("unsupported image format %q", format)
	}
	if decoded.Width < cfg.MinimumWidth {
		return ValidatedImage{}, "width_too_small", fmt.Errorf("width %d is below %d", decoded.Width, cfg.MinimumWidth)
	}
	if decoded.Height < cfg.MinimumHeight {
		return ValidatedImage{}, "height_too_small", fmt.Errorf("height %d is below %d", decoded.Height, cfg.MinimumHeight)
	}
	if decoded.Width > cfg.MaximumWidth {
		return ValidatedImage{}, "width_too_large", fmt.Errorf("width %d is above %d", decoded.Width, cfg.MaximumWidth)
	}
	if decoded.Height > cfg.MaximumHeight {
		return ValidatedImage{}, "height_too_large", fmt.Errorf("height %d is above %d", decoded.Height, cfg.MaximumHeight)
	}
	if int64(decoded.Width)*int64(decoded.Height) > cfg.MaximumDecodedPixels {
		return ValidatedImage{}, "pixel_limit_exceeded", fmt.Errorf("decoded pixel count exceeds configured maximum")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ValidatedImage{}, "decode_config_failed", err
	}
	var imageData image.Image
	if cfg.FullDecode {
		imageData, _, err = image.Decode(file)
		if err != nil {
			return ValidatedImage{}, "full_decode_failed", err
		}
	}
	result := ValidatedImage{MIMEType: mimeType, Width: decoded.Width, Height: decoded.Height, Bytes: info.Size()}
	if cfg.CalculateSHA256 {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return result, "hash_failed", err
		}
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			return result, "hash_failed", err
		}
		result.SHA256 = hex.EncodeToString(h.Sum(nil))
	}
	if cfg.CalculatePerceptualHash {
		if imageData == nil {
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return result, "hash_failed", err
			}
			imageData, _, err = image.Decode(file)
			if err != nil {
				return result, "full_decode_failed", err
			}
		}
		result.PerceptualHash = perceptualHash(imageData)
	}
	return result, "", nil
}
func mimeFor(format, extension string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	}
	switch strings.ToLower(extension) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	}
	return ""
}
func perceptualHash(img image.Image) string {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return ""
	}
	var values [64]uint32
	var total uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			px := bounds.Min.X + x*bounds.Dx()/8
			py := bounds.Min.Y + y*bounds.Dy()/8
			r, g, b, _ := img.At(px, py).RGBA()
			v := (r + g + b) / 3
			values[y*8+x] = v
			total += uint64(v)
		}
	}
	average := uint32(total / 64)
	var hash uint64
	for i, v := range values {
		if v >= average {
			hash |= 1 << uint(i)
		}
	}
	return fmt.Sprintf("%016x", hash)
}
