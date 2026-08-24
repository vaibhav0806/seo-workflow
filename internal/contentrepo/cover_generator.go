package contentrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const (
	coverWidth  = 1200
	coverHeight = 675

	ibmPlexSerifMediumURL    = "https://fonts.gstatic.com/s/ibmplexserif/v20/jizAREVNn1dOx-zrZ2X3pZvkTi3s-BIz.ttf"
	ibmPlexSerifMediumSHA256 = "7c2e5bb7e68744ad5d09db7ffa5e08e60e2ba7c34bbe01241ff1cf84c8c7f5a0"
	interMediumURL           = "https://fonts.gstatic.com/s/inter/v20/UcCO3FwrK3iLTeHuS_nVMrMxCp50SjIw2boKoduKmMEVuI6fMZg.ttf"
	interMediumSHA256        = "8c883f63b2c4157d997319f2c8bc6995ed4357ef371940d31ca159004a4aae63"
)

var coverFontHTTPClient = &http.Client{Timeout: 30 * time.Second}
var coverFontCache = struct {
	sync.Mutex
	content map[string][]byte
}{content: make(map[string][]byte)}

type GeneratedCover struct {
	URL   string
	Asset CoverAsset
}

// GenerateOpenRouterCover is kept for compatibility with manual cover tools.
// Cover rendering is now local; apiKey and model are intentionally unused.
func GenerateOpenRouterCover(ctx context.Context, apiKey string, model string, post BlogPost, assetBaseURL string) (GeneratedCover, error) {
	return GenerateDesignSystemCover(ctx, post, assetBaseURL)
}

func GenerateDesignSystemCover(ctx context.Context, post BlogPost, assetBaseURL string) (GeneratedCover, error) {
	serifTTF, err := fetchCoverFont(ctx, ibmPlexSerifMediumURL, ibmPlexSerifMediumSHA256)
	if err != nil {
		return GeneratedCover{}, fmt.Errorf("load IBM Plex Serif Medium: %w", err)
	}
	interTTF, err := fetchCoverFont(ctx, interMediumURL, interMediumSHA256)
	if err != nil {
		return GeneratedCover{}, fmt.Errorf("load Inter Medium: %w", err)
	}
	content, err := renderDesignSystemCover(post, serifTTF, interTTF)
	if err != nil {
		return GeneratedCover{}, err
	}
	return generatedDesignSystemCover(post, content, assetBaseURL), nil
}

func generatedDesignSystemCover(post BlogPost, content []byte, assetBaseURL string) GeneratedCover {
	assetPath := "covers/" + post.Slug + ".png"
	cover := GeneratedCover{
		Asset: CoverAsset{Path: assetPath, Content: content},
	}
	assetBaseURL = strings.TrimRight(strings.TrimSpace(assetBaseURL), "/")
	if assetBaseURL != "" {
		cover.URL = assetBaseURL + "/" + assetPath
	}
	return cover
}

func fetchCoverFont(ctx context.Context, fontURL string, expectedSHA256 string) ([]byte, error) {
	cacheKey := fontURL + ":" + expectedSHA256
	coverFontCache.Lock()
	cached := coverFontCache.content[cacheKey]
	coverFontCache.Unlock()
	if len(cached) > 0 {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fontURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := coverFontHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("font download status=%d", resp.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != expectedSHA256 {
		return nil, fmt.Errorf("font checksum mismatch")
	}
	coverFontCache.Lock()
	coverFontCache.content[cacheKey] = content
	coverFontCache.Unlock()
	return content, nil
}

func renderDesignSystemCover(post BlogPost, serifTTF []byte, interTTF []byte) ([]byte, error) {
	serifFont, err := opentype.Parse(serifTTF)
	if err != nil {
		return nil, fmt.Errorf("parse IBM Plex Serif Medium: %w", err)
	}
	interFont, err := opentype.Parse(interTTF)
	if err != nil {
		return nil, fmt.Errorf("parse Inter Medium: %w", err)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	drawVerticalGradient(canvas, color.NRGBA{R: 1, G: 88, B: 165, A: 255}, color.NRGBA{R: 161, G: 225, B: 249, A: 255})

	white := image.NewUniform(color.White)
	drawLogoMark(canvas, 47, 48, 60.145, 39.855, white)
	if err := drawWordmark(canvas, interFont); err != nil {
		return nil, err
	}

	watermark := image.NewRGBA(canvas.Bounds())
	drawVerticalGradientRange(
		watermark,
		251,
		625,
		color.NRGBA{R: 182, G: 225, B: 251, A: 51},
		color.NRGBA{R: 226, G: 240, B: 252, A: 51},
	)
	drawLogoMark(canvas, 765, 251, 564, 374, watermark)

	if err := drawHeadline(canvas, serifFont, coverHeadline(post)); err != nil {
		return nil, err
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return nil, fmt.Errorf("encode design-system cover: %w", err)
	}
	return encoded.Bytes(), nil
}

func coverHeadline(post BlogPost) string {
	words := strings.Fields(strings.TrimSpace(post.Title))
	if len(words) > 8 {
		words = words[:8]
	}
	return strings.Join(words, " ")
}

func drawVerticalGradient(destination *image.RGBA, top color.NRGBA, bottom color.NRGBA) {
	drawVerticalGradientRange(destination, 0, destination.Bounds().Dy()-1, top, bottom)
}

func drawVerticalGradientRange(destination *image.RGBA, startY int, endY int, top color.NRGBA, bottom color.NRGBA) {
	if endY <= startY {
		return
	}
	bounds := destination.Bounds()
	for y := max(startY, bounds.Min.Y); y <= min(endY, bounds.Max.Y-1); y++ {
		t := float64(y-startY) / float64(endY-startY)
		rowColor := color.NRGBA{
			R: uint8(float64(top.R) + (float64(bottom.R)-float64(top.R))*t),
			G: uint8(float64(top.G) + (float64(bottom.G)-float64(top.G))*t),
			B: uint8(float64(top.B) + (float64(bottom.B)-float64(top.B))*t),
			A: uint8(float64(top.A) + (float64(bottom.A)-float64(top.A))*t),
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			destination.Set(x, y, rowColor)
		}
	}
}

func drawWordmark(destination *image.RGBA, interFont *opentype.Font) error {
	face, err := opentype.NewFace(interFont, &opentype.FaceOptions{Size: 36.23, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("create Inter Medium face: %w", err)
	}
	defer face.Close()

	metrics := face.Metrics()
	textHeight := metrics.Ascent + metrics.Descent
	markTop := fixed.I(48)
	markHeight := fixed.Int26_6(2551)
	baseline := markTop + (markHeight-textHeight)/2 + metrics.Ascent
	drawTrackedString(destination, face, "CreateOS", 121.645, baseline, -0.04*36.23, color.White)
	return nil
}

func drawHeadline(destination *image.RGBA, serifFont *opentype.Font, headline string) error {
	if headline == "" {
		return fmt.Errorf("cover headline is required")
	}

	const (
		left     = 47.0
		top      = 361.0
		maxWidth = 638.0
	)
	face, err := opentype.NewFace(serifFont, &opentype.FaceOptions{Size: 82, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("create IBM Plex Serif Medium face: %w", err)
	}
	defer face.Close()

	const tracking = -0.04 * 82
	lines := wrapTrackedText(face, headline, tracking, maxWidth)
	if len(lines) > 3 {
		return fmt.Errorf("cover headline exceeds three lines at 82px")
	}
	for _, line := range lines {
		if measureTrackedString(face, line, tracking) > maxWidth {
			return fmt.Errorf("cover headline line exceeds 638px at 82px")
		}
	}

	baseline := fixed.I(int(top)) + face.Metrics().Ascent
	for index, line := range lines {
		drawTrackedString(destination, face, line, left, baseline+fixed.I(index*88), tracking, color.White)
	}
	return nil
}

func wrapTrackedText(face font.Face, text string, tracking float64, maxWidth float64) []string {
	words := strings.Fields(text)
	lines := make([]string, 0, 3)
	for _, word := range words {
		if len(lines) == 0 {
			lines = append(lines, word)
			continue
		}
		candidate := lines[len(lines)-1] + " " + word
		if measureTrackedString(face, candidate, tracking) <= maxWidth {
			lines[len(lines)-1] = candidate
			continue
		}
		lines = append(lines, word)
	}
	return lines
}

func measureTrackedString(face font.Face, text string, tracking float64) float64 {
	var width fixed.Int26_6
	var previous rune
	for index, current := range []rune(text) {
		if index > 0 {
			width += face.Kern(previous, current) + fixed.Int26_6(tracking*64)
		}
		advance, ok := face.GlyphAdvance(current)
		if ok {
			width += advance
		}
		previous = current
	}
	return float64(width) / 64
}

func drawTrackedString(destination *image.RGBA, face font.Face, text string, x float64, baseline fixed.Int26_6, tracking float64, ink color.Color) {
	drawer := font.Drawer{Dst: destination, Src: image.NewUniform(ink), Face: face, Dot: fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: baseline}}
	var previous rune
	for offset := 0; offset < len(text); {
		current, size := utf8.DecodeRuneInString(text[offset:])
		if offset > 0 {
			drawer.Dot.X += face.Kern(previous, current) + fixed.Int26_6(tracking*64)
		}
		drawer.DrawString(string(current))
		previous = current
		offset += size
	}
}

func drawLogoMark(destination *image.RGBA, left float32, top float32, width float32, height float32, source image.Image) {
	sx := width / 60.145
	sy := height / 39.855
	x := func(value float32) float32 { return left + value*sx }
	y := func(value float32) float32 { return top + value*sy }

	rasterizer := vector.NewRasterizer(destination.Bounds().Dx(), destination.Bounds().Dy())
	rasterizer.MoveTo(x(18.05), y(39.855))
	rasterizer.LineTo(x(2.042), y(39.855))
	rasterizer.CubeTo(x(0.914), y(39.855), x(0), y(38.941), x(0), y(37.813))
	rasterizer.LineTo(x(0), y(2.042))
	rasterizer.CubeTo(x(0), y(0.914), x(0.914), y(0), x(2.042), y(0))
	rasterizer.LineTo(x(38.275), y(0))
	rasterizer.CubeTo(x(39.403), y(0), x(40.317), y(0.914), x(40.317), y(2.042))
	rasterizer.LineTo(x(40.317), y(14.892))
	rasterizer.CubeTo(x(40.317), y(15.445), x(40.541), y(15.974), x(40.938), y(16.358))
	rasterizer.LineTo(x(43.821), y(19.153))
	rasterizer.CubeTo(x(44.202), y(19.522), x(44.711), y(19.729), x(45.242), y(19.729))
	rasterizer.LineTo(x(58.103), y(19.729))
	rasterizer.CubeTo(x(59.231), y(19.729), x(60.145), y(20.643), x(60.145), y(21.771))
	rasterizer.LineTo(x(60.145), y(37.548))
	rasterizer.CubeTo(x(60.145), y(38.676), x(59.231), y(39.59), x(58.103), y(39.59))
	rasterizer.LineTo(x(42.359), y(39.59))
	rasterizer.CubeTo(x(41.231), y(39.59), x(40.317), y(38.676), x(40.317), y(37.548))
	rasterizer.LineTo(x(40.317), y(24.831))
	rasterizer.CubeTo(x(40.317), y(24.278), x(40.093), y(23.749), x(39.695), y(23.364))
	rasterizer.LineTo(x(36.945), y(20.701))
	rasterizer.CubeTo(x(36.564), y(20.332), x(36.055), y(20.126), x(35.525), y(20.126))
	rasterizer.LineTo(x(22.135), y(20.126))
	rasterizer.CubeTo(x(21.007), y(20.126), x(20.092), y(21.04), x(20.092), y(22.168))
	rasterizer.LineTo(x(20.092), y(37.813))
	rasterizer.CubeTo(x(20.092), y(38.941), x(19.178), y(39.855), x(18.05), y(39.855))
	rasterizer.ClosePath()
	rasterizer.Draw(destination, destination.Bounds(), source, image.Point{})
}
