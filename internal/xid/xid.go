// Package xid generates the X-Client-Transaction-Id header.
// The algorithm follows the public x_client_transaction implementation.
// Initialization failure is not fatal: callers omit the header.
package xid

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	keyword = "obfiowerehiring"
	extra   = 3
	epochMS = 1682924400 * 1000
)

var (
	ondemandRE   = regexp.MustCompile(`['"]ondemand\.s['"]\s*:\s*['"]([\w]*)['"]`)
	ondemandIDRE = regexp.MustCompile(`(\d+):"ondemand\.s"`)
	indicesRE    = regexp.MustCompile(`\(\w\[(\d{1,2})\],\s*16\)`)
	migrateRE    = regexp.MustCompile(`https?://(?:www\.)?(?:twitter|x)\.com(?:/x)?/migrate(?:[/?])?tok=[a-zA-Z0-9%\-_]+`)
)

// Generator holds the key material parsed from the homepage and ondemand script.
type Generator struct {
	key          string
	keyBytes     []byte
	rowIndex     int
	keyIndices   []int
	animationKey string
}

// OnDemandURL extracts the ondemand script URL from homepage HTML.
// Older pages embed `"ondemand.s":"hash"`. Current pages use two webpack maps
// that share a numeric id: `59924:"ondemand.s"` and `59924:"<hash>"`.
func OnDemandURL(page string) string {
	if match := ondemandRE.FindStringSubmatch(page); match != nil && match[1] != "" {
		return ondemandScript(match[1])
	}
	id := ondemandIDRE.FindStringSubmatch(page)
	if id == nil {
		return ""
	}
	hash := regexp.MustCompile(regexp.QuoteMeta(id[1]) + `:"([0-9a-fA-F]{8,})"`).FindStringSubmatch(page)
	if hash == nil {
		return ""
	}
	return ondemandScript(hash[1])
}

func ondemandScript(hash string) string {
	return "https://abs.twimg.com/responsive-web/client-web/ondemand.s." + hash + "a.js"
}

// MigrationURL returns an x.com migration URL when the homepage is an interstitial.
func MigrationURL(page string) string {
	return migrateRE.FindString(page)
}

// New parses homepage HTML and the ondemand script.
func New(page, ondemand string) (*Generator, error) {
	indices := indicesRE.FindAllStringSubmatch(ondemand, -1)
	if len(indices) == 0 {
		return nil, fmt.Errorf("couldn't get KEY_BYTE indices")
	}
	row, err := strconv.Atoi(indices[0][1])
	if err != nil {
		return nil, err
	}
	var rest []int
	for _, item := range indices[1:] {
		n, err := strconv.Atoi(item[1])
		if err != nil {
			return nil, err
		}
		rest = append(rest, n)
	}
	key := metaContent(page, "twitter-site-verification")
	if key == "" {
		return nil, fmt.Errorf("couldn't get twitter-site-verification")
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(key)
		if err != nil {
			return nil, err
		}
	}
	gen := &Generator{key: key, keyBytes: raw, rowIndex: row, keyIndices: rest}
	gen.animationKey, err = gen.animation(page)
	if err != nil {
		return nil, err
	}
	return gen, nil
}

// ID returns the header value for one request.
func (g *Generator) ID(method, path string) string {
	now := int(math.Floor(float64(time.Now().UnixMilli()-epochMS) / 1000))
	return g.idAt(method, path, now, rand.Intn(256))
}

func (g *Generator) idAt(method, path string, now, randomByte int) string {
	var timeBytes []byte
	for i := 0; i < 4; i++ {
		timeBytes = append(timeBytes, byte((now>>(i*8))&0xFF))
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s!%s!%d%s%s", method, path, now, keyword, g.animationKey)))
	bytesArr := append([]byte{}, g.keyBytes...)
	bytesArr = append(bytesArr, timeBytes...)
	bytesArr = append(bytesArr, sum[:16]...)
	bytesArr = append(bytesArr, extra)
	out := []byte{byte(randomByte)}
	for _, item := range bytesArr {
		out = append(out, item^byte(randomByte))
	}
	return strings.TrimRight(base64.StdEncoding.EncodeToString(out), "=")
}

func (g *Generator) animation(page string) (string, error) {
	frames := loadingFrames(page)
	if len(g.keyBytes) < 6 || len(frames) == 0 {
		return "", fmt.Errorf("missing animation frames")
	}
	row := int(g.keyBytes[g.rowIndex]) % 16
	frameTime := 1
	for _, index := range g.keyIndices {
		if index < 0 || index >= len(g.keyBytes) {
			return "", fmt.Errorf("key byte index %d out of range", index)
		}
		frameTime *= int(g.keyBytes[index]) % 16
	}
	frameTime = jsRound(float64(frameTime)/10) * 10
	selected := frames[int(g.keyBytes[5])%4%len(frames)]
	rows := pathRows(selected)
	if row < 0 || row >= len(rows) {
		return "", fmt.Errorf("animation row %d missing", row)
	}
	return animate(rows[row], float64(frameTime)/4096), nil
}

func animate(frames []int, target float64) string {
	if len(frames) < 7 {
		return ""
	}
	from := []float64{float64(frames[0]), float64(frames[1]), float64(frames[2]), 1}
	to := []float64{float64(frames[3]), float64(frames[4]), float64(frames[5]), 1}
	rotationTo := solve(float64(frames[6]), 60, 360, true)
	var curves []float64
	for i, item := range frames[7:] {
		min := 0.0
		if i%2 == 1 {
			min = -1
		}
		curves = append(curves, solve(float64(item), min, 1, false))
	}
	val := cubic(curves, target)
	color := interpolate(from, to, val)
	for i := range color {
		if color[i] < 0 {
			color[i] = 0
		}
		if color[i] > 255 {
			color[i] = 255
		}
	}
	rotation := interpolate([]float64{0}, []float64{rotationTo}, val)
	matrix := rotationMatrix(rotation[0])
	var parts []string
	for _, value := range color[:len(color)-1] {
		parts = append(parts, strconv.FormatInt(int64(math.Round(value)), 16))
	}
	for _, value := range matrix {
		rounded := math.Round(math.Abs(value)*100) / 100
		hexed := floatToHex(rounded)
		if strings.HasPrefix(hexed, ".") {
			hexed = "0" + hexed
		}
		if hexed == "" {
			hexed = "0"
		}
		parts = append(parts, strings.ToLower(hexed))
	}
	parts = append(parts, "0", "0")
	return strings.NewReplacer(".", "", "-", "").Replace(strings.Join(parts, ""))
}

func solve(value, min, max float64, rounding bool) float64 {
	result := value*(max-min)/255 + min
	if rounding {
		return math.Floor(result)
	}
	return math.Round(result*100) / 100
}

func interpolate(from, to []float64, f float64) []float64 {
	out := make([]float64, len(from))
	for i := range from {
		out[i] = from[i]*(1-f) + to[i]*f
	}
	return out
}

func rotationMatrix(degrees float64) []float64 {
	rad := degrees * math.Pi / 180
	return []float64{math.Cos(rad), -math.Sin(rad), math.Sin(rad), math.Cos(rad)}
}

func cubic(curves []float64, t float64) float64 {
	if len(curves) < 4 {
		return t
	}
	if t <= 0 {
		var start float64
		if curves[0] > 0 {
			start = curves[1] / curves[0]
		} else if curves[1] == 0 && curves[2] > 0 {
			start = curves[3] / curves[2]
		}
		return start * t
	}
	if t >= 1 {
		var end float64
		if curves[2] < 1 {
			end = (curves[3] - 1) / (curves[2] - 1)
		} else if curves[2] == 1 && curves[0] < 1 {
			end = (curves[1] - 1) / (curves[0] - 1)
		}
		return 1 + end*(t-1)
	}
	start, end := 0.0, 1.0
	var mid float64
	for i := 0; i < 64 && start < end; i++ {
		mid = (start + end) / 2
		est := cubicCalc(curves[0], curves[2], mid)
		if math.Abs(t-est) < 0.00001 {
			return cubicCalc(curves[1], curves[3], mid)
		}
		if est < t {
			start = mid
		} else {
			end = mid
		}
	}
	return cubicCalc(curves[1], curves[3], mid)
}

func cubicCalc(a, b, m float64) float64 {
	return 3*a*(1-m)*(1-m)*m + 3*b*(1-m)*m*m + m*m*m
}

func jsRound(num float64) int {
	x := math.Floor(num)
	if num-x >= 0.5 {
		x = math.Ceil(num)
	}
	return int(math.Copysign(x, num))
}

func floatToHex(x float64) string {
	var result []byte
	quotient := int(x)
	fraction := x - float64(quotient)
	for quotient > 0 {
		next := int(x / 16)
		remainder := int(x - float64(next)*16)
		if remainder > 9 {
			result = append([]byte{byte(remainder + 55)}, result...)
		} else {
			result = append([]byte{byte('0' + remainder)}, result...)
		}
		x = float64(next)
		quotient = next
	}
	if fraction == 0 {
		return string(result)
	}
	result = append(result, '.')
	for fraction > 0 && len(result) < 20 {
		fraction *= 16
		integer := int(fraction)
		fraction -= float64(integer)
		if integer > 9 {
			result = append(result, byte(integer+55))
		} else {
			result = append(result, byte('0'+integer))
		}
	}
	return string(result)
}

func metaContent(page, name string) string {
	z := html.NewTokenizer(strings.NewReader(page))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if tok.Data != "meta" {
				continue
			}
			var metaName, content string
			for _, attr := range tok.Attr {
				switch attr.Key {
				case "name":
					metaName = attr.Val
				case "content":
					content = attr.Val
				}
			}
			if metaName == name {
				return content
			}
		}
	}
}

func loadingFrames(page string) []*html.Node {
	root, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	var frames []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "id" && strings.HasPrefix(attr.Val, "loading-x-anim") {
					frames = append(frames, n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return frames
}

func pathRows(frame *html.Node) [][]int {
	first := elementChild(frame, 0)
	if first == nil {
		return nil
	}
	second := elementChild(first, 1)
	if second == nil {
		return nil
	}
	var d string
	for _, attr := range second.Attr {
		if attr.Key == "d" {
			d = attr.Val
		}
	}
	if len(d) < 9 {
		return nil
	}
	var rows [][]int
	for _, item := range strings.Split(d[9:], "C") {
		fields := strings.Fields(regexp.MustCompile(`[^\d]+`).ReplaceAllString(item, " "))
		var row []int
		for _, field := range fields {
			n, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			row = append(row, n)
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func elementChild(n *html.Node, index int) *html.Node {
	seen := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if seen == index {
			return c
		}
		seen++
	}
	return nil
}
