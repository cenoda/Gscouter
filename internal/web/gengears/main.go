// Command gengears writes templates/sky_gears.html: the two gear trains
// behind the page, part of the draft sky in templates/sky.html. Run it
// from internal/web after changing the trains below.
//
// Every gear shares one tooth size (the module), so a gear of n teeth
// has a pitch radius of module*n/2 and any two can mesh:
//
//   - Two outside gears sit the sum of their pitch radii apart and turn
//     opposite ways.
//   - A gear inside a ring's inward teeth sits the difference apart and
//     turns the ring's way.
//   - Two sprockets joined by a chain turn the same way, and the chain's
//     roller pitch is the tooth pitch. A chain wraps the far side of its
//     sprockets, so a driven one sits on another gear's axle.
//
// A gear's speed is its neighbour's times the ratio of their teeth, and
// its start angle puts its gaps where the neighbour's teeth are. Teeth are
// slimmer than gaps, so in motion no two ever touch. Missing, chipped and
// notched teeth are only drawn: the motion is the same.
package main

import (
	"cmp"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"strings"
)

const module = 2.5

// Tooth shape: half-widths at the root and the tip as fractions of the
// angle between teeth, and the tip's height past the pitch circle in
// modules. The root is 1.25 modules the other side.
const toothRoot, toothTop, toothTip = .18, .075, .5

// rollerPitch is the chain's link length, equal to the tooth pitch.
const rollerPitch = math.Pi * module

type gear struct {
	n, inner int     // outside teeth, inward teeth (a ring)
	x, y     float64 // centre, px from the train's anchor
	phase    float64 // start angle, degrees
	speed    float64 // degrees a second, clockwise
	look
}

// look is how a gear is drawn; the zero look is a whole gear.
type look struct {
	spokes        int
	missing, chip []int // tooth numbers, 0 at the start angle
	notch, crack  *float64
	plain         bool // no spokes, under a sprocket on the same axle
}

type kind int

const (
	outside kind = iota // meshes outside the parent's teeth
	inside              // sits within the parent ring's inward teeth
	chained             // a sprocket the parent drives by chain
	axle                // fixed on the parent's axle, drawn over it
)

// link adds a gear of n teeth (and inner inward ones, for a ring) at
// angle degrees from gear parent's centre.
type link struct {
	parent, n, inner int
	angle            float64
	kind             kind
	look
}

type chain struct {
	a, b  int    // sprockets
	path  string // roller path, starting on a roller
	plate [2]string
	speed float64 // seconds per link
	box   [4]float64
}

func pitch(n int) float64 { return module * float64(n) / 2 }

func mod(a, b float64) float64 { return a - b*math.Floor(a/b) }

func rad(deg float64) float64 { return deg * math.Pi / 180 }

func deg(r float64) float64 { return r * 180 / math.Pi }

func ptAt(x, y, d, r float64) (float64, float64) {
	return x + r*math.Cos(rad(d)), y + r*math.Sin(rad(d))
}

type train struct {
	gears  []gear
	chains []chain
}

// newTrain starts a train with a driver of n teeth (inner inward ones)
// taking period seconds a turn, counter-clockwise if ccw.
func newTrain(n, inner int, period float64, ccw bool, lk look) *train {
	s := 360 / period
	if ccw {
		s = -s
	}
	return &train{gears: []gear{{n: n, inner: inner, speed: s, look: lk}}}
}

func (t *train) add(l link) {
	a := t.gears[l.parent]
	g := gear{n: l.n, inner: l.inner, look: l.look}
	pb := 360 / float64(l.n)
	switch l.kind {
	case outside:
		// In teeth from the contact point, the two gears' positions add
		// up to a half: a tooth meets a gap. Rolling keeps the sum.
		d := pitch(a.n) + pitch(l.n)
		g.x, g.y = ptAt(a.x, a.y, l.angle, d)
		sa := mod((l.angle-a.phase)*float64(a.n)/360, 1)
		g.phase = mod(l.angle+180-pb*(.5-sa), 360)
		g.speed = -a.speed * float64(a.n) / float64(l.n)
	case inside:
		// Both turn the same way, so here the difference is kept.
		d := pitch(a.inner) - pitch(l.n)
		g.x, g.y = ptAt(a.x, a.y, l.angle, d)
		sr := mod((l.angle-a.phase)*float64(a.inner)/360, 1)
		g.phase = mod(l.angle-pb*(sr-.5), 360)
		g.speed = a.speed * float64(a.inner) / float64(l.n)
	case axle:
		g.x, g.y, g.speed = a.x, a.y, a.speed
	case chained:
		g.speed = a.speed * float64(a.n) / float64(l.n)
		t.gears = append(t.gears, g)
		t.chain(l.parent, len(t.gears)-1, l.angle)
		return
	}
	t.gears = append(t.gears, g)
}

// chain places sprocket b along angle from sprocket a, at the distance
// that makes the chain a whole number of links, and lays the chain.
func (t *train) chain(ai, bi int, angle float64) {
	a, b := &t.gears[ai], &t.gears[bi]
	r1, r2 := pitch(a.n), pitch(b.n)
	length := func(d float64) float64 {
		wrap := 2 * math.Acos((r1-r2)/d) // round the far sprocket
		return 2*math.Sqrt(d*d-(r1-r2)*(r1-r2)) + r2*wrap + r1*(2*math.Pi-wrap)
	}
	// About five links between the sprockets, rounded to a whole chain.
	d0 := r1 + r2 + 5*rollerPitch
	links := math.Round(length(d0) / rollerPitch)
	lo, hi := math.Abs(r1-r2)+1, 10*d0
	for range 100 {
		if mid := (lo + hi) / 2; length(mid) < links*rollerPitch {
			lo = mid
		} else {
			hi = mid
		}
	}
	d := lo
	b.x, b.y = ptAt(a.x, a.y, angle, d)

	// Clockwise round both; the normals point from each centre to the
	// straight runs.
	nx := (r1 - r2) / d
	up := angle + deg(math.Atan2(-math.Sqrt(1-nx*nx), nx))
	down := angle + deg(math.Atan2(math.Sqrt(1-nx*nx), nx))
	if a.speed < 0 {
		log.Fatal("chains run clockwise")
	}
	// The rollers start in a gap of a, on a's half of the loop.
	step := 360 / float64(a.n)
	start := a.phase + (math.Floor((angle+180-a.phase)/step)+.5)*step
	for start < down {
		start += step
	}
	for start > up+360 {
		start -= step
	}
	loop := func(dr float64) string {
		p := func(x, y, d, r float64) string {
			px, py := ptAt(x, y, d, r)
			return fmt.Sprintf("%.2f %.2f", px, py)
		}
		arc := func(r, sweep float64, x, y, d float64) string {
			return fmt.Sprintf("A%.2f %.2f 0 %d 1 %s", r, r, b2i(sweep > 180), p(x, y, d, r))
		}
		ra, rb := r1+dr, r2+dr
		return "M" + p(a.x, a.y, start, ra) +
			arc(ra, up+360-start, a.x, a.y, up) +
			"L" + p(b.x, b.y, up, rb) +
			arc(rb, down-up, b.x, b.y, down) +
			"L" + p(a.x, a.y, down, ra) +
			arc(ra, start-down, a.x, a.y, start)
	}
	// The first roller on b's arc sets b's phase: a gap goes under it.
	run := math.Sqrt(d*d - (r1-r2)*(r1-r2))
	onB := r1*rad(up+360-start) + run
	k := math.Ceil(onB / rollerPitch)
	b.phase = mod(up+deg((k*rollerPitch-onB)/r2)-180/float64(b.n), 360)
	w := 2.6 // half the plates' width
	ax, ay := a.x, a.y
	bx, by := b.x, b.y
	m := math.Max(r1, r2) + w + 2
	t.chains = append(t.chains, chain{
		a: ai, b: bi, path: loop(0), plate: [2]string{loop(-w), loop(w)},
		speed: 360 / float64(a.n) / a.speed,
		box:   [4]float64{math.Min(ax, bx) - m, math.Min(ay, by) - m, math.Max(ax, bx) + m, math.Max(ay, by) + m},
	})
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func outerReach(g gear) float64 { return pitch(g.n) + toothTip*module }

// check fails unless the only gears near each other are those that mesh.
func (t *train) check() {
	const margin = 2*module + 4
	for i, g := range t.gears {
		for _, h := range t.gears[:i] {
			dist := math.Hypot(g.x-h.x, g.y-h.y)
			near := func(v float64) bool { return math.Abs(dist-v) < 1e-6 }
			if dist < 1e-6 || near(pitch(g.n)+pitch(h.n)) || near(pitch(g.inner)-pitch(h.n)) || near(pitch(h.inner)-pitch(g.n)) {
				continue
			}
			inRing := func(ring, in gear) bool {
				return ring.inner > 0 && dist+outerReach(in)+margin < pitch(ring.inner)-toothTip*module
			}
			if inRing(g, h) || inRing(h, g) || dist > outerReach(g)+outerReach(h)+margin {
				continue
			}
			log.Fatalf("gears of %d and %d teeth collide", g.n, h.n)
		}
	}
}

func num(v float64) string { return fmt.Sprintf("%.6g", v) }

func pt(d, r float64) string {
	return fmt.Sprintf("%.1f %.1f", r*math.Cos(rad(d)), r*math.Sin(rad(d)))
}

// teeth outlines n teeth on a pitch circle of radius r, pointing outward
// or, for a ring, inward. A missing tooth leaves the root circle, a
// chipped one stops short and a notch bites into the rim.
func teeth(n int, inward bool, lk look) string {
	r := pitch(n)
	root, tip, step := r-1.25*module, r+toothTip*module, 360/float64(n)
	if inward {
		root, tip = r+1.25*module, r-toothTip*module
	}
	var pts []string
	for k := range n {
		a := float64(k) * step
		off := 360.0
		if lk.notch != nil {
			off = mod(a-*lk.notch+180, 360) - 180
		}
		switch {
		case math.Abs(off) < step*1.6:
			depth := root - 6 + math.Abs(off)/step*3
			pts = append(pts, pt(a-toothRoot*step, depth), pt(a+toothRoot*step, depth))
		case slices.Contains(lk.missing, k):
			pts = append(pts, pt(a-toothRoot*step, root), pt(a, root), pt(a+toothRoot*step, root))
		default:
			t := tip
			if slices.Contains(lk.chip, k) {
				t = r + .2*module
			}
			pts = append(pts, pt(a-toothRoot*step, root), pt(a-toothTop*step, t),
				pt(a+toothTop*step, t), pt(a+toothRoot*step, root))
		}
	}
	return "M" + strings.Join(pts, "L") + "Z"
}

// body draws a gear in Celestia's manner (filigree spokes, round
// cut-outs, a star at the hub) or the Abyss's (wedge cut-outs, a dashed
// rim, a triangle at the hub).
func body(g gear, abyss bool) string {
	var b strings.Builder
	r := pitch(g.n)
	root := r - 1.25*module
	d := teeth(g.n, false, g.look)
	inner := root - 5
	if g.inner > 0 {
		d += teeth(g.inner, true, look{})
	}
	fmt.Fprintf(&b, `<path d="%s" fill="currentColor" fill-opacity=".3" fill-rule="evenodd"/>`, d)
	if g.inner > 0 {
		// A ring: decorate the band between its two sets of teeth.
		mid := (pitch(g.inner) + 1.25*module + root) / 2
		if abyss {
			fmt.Fprintf(&b, `<circle r="%s" stroke-dasharray="6 3 1 3" stroke-opacity=".7"/>`, num(mid))
		} else {
			var ticks strings.Builder
			for i := range 24 {
				a := float64(i) * 15
				l := 1.5
				if i%3 == 0 {
					l = 3
				}
				fmt.Fprintf(&ticks, "M%sL%s", pt(a, mid-l), pt(a, mid+l))
			}
			fmt.Fprintf(&b, `<circle r="%s" stroke-opacity=".5"/><path d="%s" stroke-opacity=".7"/>`, num(mid), ticks.String())
		}
		return b.String()
	}
	hub := math.Max(5, r*.2)
	if r > 30 && !g.plain {
		spokes := cmp.Or(g.spokes, 5)
		var sp strings.Builder
		if abyss {
			fmt.Fprintf(&b, `<circle r="%s" stroke-dasharray="5 2" stroke-opacity=".6"/>`, num(inner))
			w := 180 / float64(spokes) * .45
			for i := range spokes {
				a := float64(i) * 360 / float64(spokes)
				fmt.Fprintf(&sp, "M%sL%sL%sZ", pt(a, hub+3), pt(a-w, inner-3), pt(a+w, inner-3))
			}
		} else {
			fmt.Fprintf(&b, `<circle r="%s" stroke-opacity=".6"/><circle r="%s" stroke-opacity=".35"/>`, num(inner), num(inner-3))
			for i := range spokes {
				a := float64(i) * 360 / float64(spokes)
				fmt.Fprintf(&sp, "M%sQ%s %s", pt(a, hub), pt(a+14, (hub+inner)/2), pt(a+24, inner-3))
				hole := (inner - hub) * .2
				fmt.Fprintf(&sp, "M%sa%.1f %.1f 0 1 0 0.1 0", pt(a+180/float64(spokes)+8, (hub+inner)*.55), hole, hole)
			}
		}
		fmt.Fprintf(&b, `<path d="%s" stroke-opacity=".6"/>`, sp.String())
	}
	fmt.Fprintf(&b, `<circle r="%s"/>`, num(hub))
	if abyss {
		fmt.Fprintf(&b, `<path d="M%sL%sL%sZ" fill="currentColor" fill-opacity=".35"/>`, pt(90, hub*.75), pt(210, hub*.75), pt(330, hub*.75))
	} else {
		var star []string
		for i := range 8 {
			rr := hub * .85
			if i%2 == 1 {
				rr = hub * .3
			}
			star = append(star, pt(float64(i)*45, rr))
		}
		fmt.Fprintf(&b, `<path d="M%sZ" fill="currentColor" fill-opacity=".45" stroke="none"/>`, strings.Join(star, "L"))
	}
	if g.crack != nil {
		var pts []string
		for i, f := range []float64{1, .8, .62, .45} {
			a := *g.crack - 3
			if i%2 == 1 {
				a = *g.crack + 4
			}
			pts = append(pts, pt(a, root*f))
		}
		fmt.Fprintf(&b, `<path d="M%s"/>`, strings.Join(pts, " L"))
	}
	return b.String()
}

// emit writes a train anchored at the given CSS position; class picks the
// Celestia (c) or Abyss (a) look.
// tones are the gears' brightness in turn, so that like the gears in the
// game's clock they read as near and far layers.
var tones = []float64{1, .55, .85, .4, .7, .95, .5, .8, .6}

func emit(b *strings.Builder, anchor, class string, t *train) {
	t.check()
	fmt.Fprintf(b, "<div class=\"gears\" style=\"%s\">\n", anchor)
	for i, g := range t.gears {
		ra := math.Max(pitch(g.n), pitch(g.inner)) + module + 1
		c := class
		if g.speed < 0 {
			c += " ccw"
		}
		fmt.Fprintf(b, `<svg class="gear %s" viewBox="%.1f %.1f %.1f %.1f" style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;animation-duration:%ss;--tone:%s"><g transform="rotate(%.3f)">%s</g></svg>`+"\n",
			c, -ra, -ra, 2*ra, 2*ra, g.x-ra, g.y-ra, 2*ra, 2*ra, num(360/math.Abs(g.speed)), num(tones[i%len(tones)]), g.phase, body(g, class == "a"))
	}
	for _, ch := range t.chains {
		x0, y0, x1, y1 := ch.box[0], ch.box[1], ch.box[2], ch.box[3]
		fmt.Fprintf(b, `<svg class="belt %s" viewBox="%.1f %.1f %.1f %.1f" style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;--tone:%s"><path d="%s%s" stroke-opacity=".6"/><path class="chain" d="%s" style="stroke-dasharray:0 %.5f;animation-duration:%ss"/></svg>`+"\n",
			class, x0, y0, x1-x0, y1-y0, x0, y0, x1-x0, y1-y0, num(tones[ch.b%len(tones)]), ch.plate[0], ch.plate[1], ch.path, rollerPitch, num(ch.speed))
	}
	b.WriteString("</div>\n")
}

func at(v float64) *float64 { return &v }

func main() {
	// Celestia, bottom right: a ring whose inward teeth turn three
	// planets round a sun, driving a train of outside gears.
	cel := newTrain(64, 48, 120, false, look{})
	for _, a := range []float64{-90, 30, 150} {
		cel.add(link{parent: 0, n: 18, angle: a, kind: inside})
	}
	cel.add(link{parent: 1, n: 12, angle: 90})
	cel.add(link{parent: 0, n: 20, angle: 185, look: look{missing: []int{3, 4}}})
	cel.add(link{parent: 0, n: 36, angle: 262, look: look{spokes: 4, notch: at(40), crack: at(200)}})
	cel.add(link{parent: 6, n: 24, angle: 195, look: look{chip: []int{7}}})
	cel.add(link{parent: 5, n: 16, angle: 235})
	cel.add(link{parent: 7, n: 40, angle: 255, look: look{missing: []int{30}}})

	// The Abyss, left middle: a sprocket chained off a gear's axle, and a
	// ring with a pinion running inside it.
	aby := newTrain(48, 0, 96, true, look{missing: []int{10}})
	aby.add(link{parent: 0, n: 18, angle: -50, look: look{chip: []int{0}}})
	aby.add(link{parent: 0, n: 30, angle: 40, look: look{plain: true, crack: at(60)}})
	aby.add(link{parent: 2, n: 14, kind: axle})
	aby.add(link{parent: 3, n: 12, angle: -15, kind: chained})
	aby.add(link{parent: 2, n: 40, inner: 28, angle: 95, look: look{notch: at(150), missing: []int{20, 21, 22}}})
	aby.add(link{parent: 5, n: 12, angle: 200, kind: inside})

	var b strings.Builder
	b.WriteString("{{/* Generated by gengears; do not edit. */}}\n{{define \"sky-gears\"}}")
	emit(&b, "right:30px;bottom:40px", "c", cel)
	emit(&b, "left:-20px;top:44%", "a", aby)
	b.WriteString("{{end}}\n")
	if err := os.WriteFile("templates/sky_gears.html", []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}
