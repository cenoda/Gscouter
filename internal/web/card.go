package web

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Cards are the images link previews show for a result: 1200x630, the
// Open Graph size. Text is ASCII only (the Go fonts have no Hangul), so
// nicknames stay in the preview's title.
const cardW, cardH = 1200, 630

var (
	cardBg     = rgb(0x11141a)
	cardText   = rgb(0xe6e9ef)
	cardMuted  = rgb(0x9aa3b2)
	cardLine   = rgb(0x2c3340)
	cardAccent = rgb(0x7aa2f7)
	cardGood   = rgb(0x5fd39a)
	// elementColors are the site's dark-theme element colors.
	elementColors = map[string]color.RGBA{
		"pyro": rgb(0xf27f5c), "hydro": rgb(0x5eaee8), "anemo": rgb(0x55cfae), "electro": rgb(0xb88af0),
		"dendro": rgb(0x86c75a), "cryo": rgb(0x7fd3e6), "geo": rgb(0xe0b452),
	}
)

func rgb(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

func (s *Server) handleScoreCard(w http.ResponseWriter, r *http.Request) {
	sel, err := parseSelection(r.URL.Query().Get("a"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	enemy, err := enemyFromQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.serveCard(w, r, func(ctx context.Context) (image.Image, bool) {
		tr := s.evaluate(ctx, english, sel, enemy)
		if tr.Result == nil {
			return nil, false
		}
		return scoreCard(ctx, tr), true
	})
}

func (s *Server) handleVersusCard(w http.ResponseWriter, r *http.Request) {
	a, errA := parseSelection(r.URL.Query().Get("a"))
	b, errB := parseSelection(r.URL.Query().Get("b"))
	if errA != nil || errB != nil {
		http.Error(w, "bad selection", http.StatusBadRequest)
		return
	}
	enemy, err := enemyFromQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.serveCard(w, r, func(ctx context.Context) (image.Image, bool) {
		d := s.versus(ctx, english, a, b, enemy)
		if !d.Scored {
			return nil, false
		}
		return versusCard(ctx, d), true
	})
}

// cardMu draws one card at a time: font faces are not safe for
// concurrent use.
var cardMu sync.Mutex

// serveCard serves a card PNG, drawing it at most once per URL a day.
func (s *Server) serveCard(w http.ResponseWriter, r *http.Request, draw func(context.Context) (image.Image, bool)) {
	key := r.URL.String()
	body, ok := s.cards.get(key)
	if !ok {
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		img, ok := draw(ctx)
		if !ok {
			http.NotFound(w, r)
			return
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			slog.Error("encode card", "err", err)
			http.Error(w, "encode failed", http.StatusInternalServerError)
			return
		}
		body = buf.Bytes()
		s.cards.put(key, body, 24*time.Hour)
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(body)
}

// cardMember is one character as a card shows it.
type cardMember struct {
	Key, Name, Element string // Name is the English in-game name
	Cons, Refine       int
	Share              float64
}

// cardMembers lists a team's members by damage share, highest first.
func cardMembers(tr teamResult) []cardMember {
	var out []cardMember
	for i, c := range tr.Result.Characters {
		m := cardMember{Key: c.Name, Name: english.charName(c.Name), Element: element(c.Name), Share: c.Share}
		if i < len(tr.Chars) {
			m.Cons, m.Refine = tr.Chars[i].Cons, tr.Chars[i].Weapon.Refine
		}
		out = append(out, m)
	}
	slices.SortStableFunc(out, func(a, b cardMember) int { return cmp.Compare(b.Share, a.Share) })
	return out
}

// scoreCard shows the score above the four members, each with their
// damage share.
func scoreCard(ctx context.Context, tr teamResult) image.Image {
	cardMu.Lock()
	defer cardMu.Unlock()
	c := newCanvas()
	c.text("GSCOUTER", 64, 92, 30, true, cardAccent)
	x := c.text(thousands(tr.Result.DPS), 60, 230, 132, true, cardText)
	c.text("DPS", x+20, 230, 52, true, cardMuted)
	sub := "Looped rotation, Lv.100 enemy"
	if tr.Enemy != nil {
		sub = fmt.Sprintf("Looped rotation, custom Lv.%d enemy", tr.Enemy.Level)
	}
	if tr.OneRotation != nil {
		sub += "  ·  one rotation " + thousands(tr.OneRotation.DPS)
	}
	c.text(sub, 64, 284, 28, false, cardMuted)

	const colW = (cardW - 2*48) / 4
	for i, m := range cardMembers(tr) {
		cx := 48 + colW*i + colW/2
		c.face(ctx, m, cx, 400, 150)
		c.centered(m.Name, cx, 520, 32, true, cardText)
		c.centered(fmt.Sprintf("C%d  R%d", m.Cons, m.Refine), cx, 556, 24, false, cardMuted)
		c.bar(cx-100, 574, 200, 12, m.Share, elementColor(m.Element))
		c.text(fmt.Sprintf("%.0f%%", m.Share*100), cx+108, 588, 24, true, cardText)
	}
	return c.img
}

// versusCard shows both teams side by side with the winner's lead.
func versusCard(ctx context.Context, d versusData) image.Image {
	cardMu.Lock()
	defer cardMu.Unlock()
	c := newCanvas()
	c.text("GSCOUTER", 64, 92, 30, true, cardAccent)
	if e := cardEnemy(d.A.Enemy); e != "" {
		c.text(e, 260, 92, 24, false, cardMuted)
	}
	c.fill(599, 60, 2, 510, cardLine)
	c.fill(570, 290, 60, 50, cardBg)
	c.centered("VS", 600, 327, 34, true, cardMuted)
	for side, tr := range []teamResult{d.A, d.B} {
		x0 := 48 + side*cardW/2
		mid := x0 + (cardW/2-96)/2
		lead := d.Diff
		if side == 1 {
			lead = -lead
		}
		dpsColor := cardText
		if !d.Tie && lead < 0 {
			dpsColor = cardMuted
		}
		dps := thousands(tr.Result.DPS)
		w := measure(dps, 96, true) + 16 + measure("DPS", 36, true)
		x := c.text(dps, mid-w/2, 236, 96, true, dpsColor)
		c.text("DPS", x+16, 236, 36, true, cardMuted)
		switch {
		case d.Tie:
			c.centered("TIE", mid, 290, 32, true, cardMuted)
		case lead > 0:
			c.centered(fmt.Sprintf("+%.1f%%", lead*100), mid, 290, 32, true, cardGood)
		}
		const step = 124
		for i, m := range cardMembers(tr) {
			cx := mid + (i*2-3)*step/2
			c.face(ctx, m, cx, 396, 108)
			c.centered(m.Name, cx, 490, 22, true, cardText)
			c.centered(fmt.Sprintf("C%d  %.0f%%", m.Cons, m.Share*100), cx, 522, 22, false, cardMuted)
			c.bar(cx-44, 538, 88, 10, m.Share, elementColor(m.Element))
		}
	}
	return c.img
}

func elementColor(e string) color.RGBA {
	if c, ok := elementColors[e]; ok {
		return c
	}
	return cardAccent
}

type canvas struct{ img *image.RGBA }

func newCanvas() *canvas {
	img := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	draw.Draw(img, img.Bounds(), image.NewUniform(cardBg), image.Point{}, draw.Src)
	return &canvas{img}
}

func (c *canvas) fill(x, y, w, h int, col color.Color) {
	draw.Draw(c.img, image.Rect(x, y, x+w, y+h), image.NewUniform(col), image.Point{}, draw.Over)
}

// bar draws a rounded progress bar filled to frac.
func (c *canvas) bar(x, y, w, h int, frac float64, col color.Color) {
	c.round(x, y, w, h, cardLine)
	if fw := int(math.Round(math.Max(0, math.Min(frac, 1)) * float64(w))); fw > 0 {
		c.round(x, y, max(fw, h), h, col)
	}
}

// round fills a rectangle with fully rounded ends.
func (c *canvas) round(x, y, w, h int, col color.Color) {
	r := float32(h) / 2
	z := vector.NewRasterizer(cardW, cardH)
	fx, fy, fw, fh := float32(x), float32(y), float32(w), float32(h)
	z.MoveTo(fx+r, fy)
	z.LineTo(fx+fw-r, fy)
	arc(z, fx+fw-r, fy+r, r, -math.Pi/2, math.Pi/2)
	z.LineTo(fx+r, fy+fh)
	arc(z, fx+r, fy+r, r, math.Pi/2, 3*math.Pi/2)
	z.ClosePath()
	z.Draw(c.img, c.img.Bounds(), image.NewUniform(col), image.Point{})
}

func circle(cx, cy, r float32) *vector.Rasterizer {
	z := vector.NewRasterizer(cardW, cardH)
	z.MoveTo(cx+r, cy)
	arc(z, cx, cy, r, 0, 2*math.Pi)
	z.ClosePath()
	return z
}

// arc continues the path along a circle from angle a0 to a1.
func arc(z *vector.Rasterizer, cx, cy, r float32, a0, a1 float64) {
	const steps = 48
	for i := 1; i <= steps; i++ {
		a := a0 + (a1-a0)*float64(i)/steps
		z.LineTo(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
	}
}

// face draws a member's face icon in an element-colored ring, or their
// initial when the icon can't be loaded.
func (c *canvas) face(ctx context.Context, m cardMember, cx, cy, d int) {
	col := elementColor(m.Element)
	r := float32(d) / 2
	circle(float32(cx), float32(cy), r+5).Draw(c.img, c.img.Bounds(), image.NewUniform(col), image.Point{})
	circle(float32(cx), float32(cy), r).Draw(c.img, c.img.Bounds(), image.NewUniform(cardLine), image.Point{})
	icon := faceImage(ctx, face(m.Key))
	if icon == nil {
		c.centered(strings.ToUpper(m.Name[:1]), cx, cy+d/6, float64(d)/2, true, cardText)
		return
	}
	// Scale the icon into a square layer, then paint it through a
	// circular mask.
	layer := image.NewRGBA(c.img.Bounds())
	draw.CatmullRom.Scale(layer, image.Rect(cx-d/2, cy-d/2, cx+d/2, cy+d/2), icon, icon.Bounds(), draw.Over, nil)
	mask := image.NewAlpha(c.img.Bounds())
	circle(float32(cx), float32(cy), r).Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(c.img, c.img.Bounds(), layer, image.Point{}, mask, image.Point{}, draw.Over)
}

// text draws s with its baseline at (x, y) and returns where it ends.
func (c *canvas) text(s string, x, y int, size float64, bold bool, col color.Color) int {
	d := font.Drawer{Dst: c.img, Src: image.NewUniform(col), Face: fontFace(size, bold), Dot: fixed.P(x, y)}
	d.DrawString(s)
	return d.Dot.X.Round()
}

func (c *canvas) centered(s string, cx, y int, size float64, bold bool, col color.Color) {
	c.text(s, cx-measure(s, size, bold)/2, y, size, bold, col)
}

func measure(s string, size float64, bold bool) int {
	return font.MeasureString(fontFace(size, bold), s).Round()
}

var (
	fontsOnce             sync.Once
	regularFont, boldFont *opentype.Font
	faces                 sync.Map // fontKey -> font.Face
)

type fontKey struct {
	size float64
	bold bool
}

func fontFace(size float64, bold bool) font.Face {
	fontsOnce.Do(func() {
		regularFont, _ = opentype.Parse(goregular.TTF)
		boldFont, _ = opentype.Parse(gobold.TTF)
	})
	k := fontKey{size, bold}
	if f, ok := faces.Load(k); ok {
		return f.(font.Face)
	}
	f := regularFont
	if bold {
		f = boldFont
	}
	ff, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err) // the embedded fonts always parse
	}
	faces.Store(k, ff)
	return ff
}

var (
	iconMu    sync.Mutex
	iconCache = map[string]image.Image{} // URL -> decoded icon, nil if it failed
)

// faceImage downloads and caches a face icon, or returns nil.
func faceImage(ctx context.Context, url string) image.Image {
	if url == "" {
		return nil
	}
	iconMu.Lock()
	img, ok := iconCache[url]
	iconMu.Unlock()
	if ok {
		return img
	}
	img = fetchImage(ctx, url)
	if img != nil {
		iconMu.Lock()
		iconCache[url] = img
		iconMu.Unlock()
	}
	return img
}

func fetchImage(ctx context.Context, url string) image.Image {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Gscouter (+https://github.com/cenoda/gscouter)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("face icon", "url", url, "err", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("face icon", "url", url, "status", resp.StatusCode)
		return nil
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		slog.Warn("face icon", "url", url, "err", err)
		return nil
	}
	return img
}
