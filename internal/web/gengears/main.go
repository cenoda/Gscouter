// Command gengears writes templates/sky_gears.html: the two gear trains
// behind the page, part of the draft sky in templates/sky.html. Run it
// from internal/web after changing the trains below.
//
// Meshing gears share one tooth size (the module), sit one pitch radius
// apart and turn the other way, each taking time in proportion to its
// teeth. A gear's start angle puts its gap where its neighbour's tooth
// is, and teeth are slimmer than gaps, so in motion no two ever touch.
// Missing, chipped and notched teeth are only drawn: the motion is the
// same.
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

// module is the tooth size in pixels: a gear of n teeth has a pitch
// radius of module*n/2.
const module = 2.5

// Tooth shape: half-widths at the root and the tip as fractions of the
// angle between teeth, and the tip's height above the pitch circle in
// modules.
const toothRoot, toothTop, toothTip = .19, .085, .55

type gear struct {
	n             int
	x, y          float64 // centre, px from the train's anchor
	phase         float64 // start angle, degrees
	ccw           bool
	period        float64 // seconds a turn takes
	spokes        int
	missing, chip []int // tooth numbers, 0 at the start angle
	notch, crack  float64
	hasNotch      bool
	hasCrack      bool
}

// look is how a gear is drawn; zero values draw a whole gear.
type look struct {
	spokes        int
	missing, chip []int
	notch, crack  *float64
}

// link meshes a new gear of n teeth with gear parent, at angle degrees
// from the parent's centre.
type link struct {
	parent, n int
	angle     float64
}

func pitch(n int) float64 { return module * float64(n) / 2 }

func mod(a, b float64) float64 { return a - b*math.Floor(a/b) }

func rad(deg float64) float64 { return deg * math.Pi / 180 }

// train lays out a driver of n teeth and the gears linked to it.
func train(n int, period float64, ccw bool, links []link, looks []look) []gear {
	gs := []gear{{n: n, period: period, ccw: ccw}}
	for _, l := range links {
		a := gs[l.parent]
		d := pitch(a.n) + pitch(l.n)
		pa, pb := 360/float64(a.n), 360/float64(l.n)
		// Measured in teeth from the contact point, a's and the new
		// gear's positions add up to a half: a tooth meets a gap. Rolling
		// keeps the sum, so the start angle is all it takes.
		sa := mod((l.angle-a.phase)/pa, 1)
		gs = append(gs, gear{
			n:      l.n,
			x:      a.x + d*math.Cos(rad(l.angle)),
			y:      a.y + d*math.Sin(rad(l.angle)),
			phase:  mod(l.angle+180-pb*(.5-sa), 360),
			ccw:    !a.ccw,
			period: a.period * float64(l.n) / float64(a.n),
		})
	}
	for i := range gs {
		lk := looks[i]
		g := &gs[i]
		g.spokes, g.missing, g.chip = cmp.Or(lk.spokes, 5), lk.missing, lk.chip
		if lk.notch != nil {
			g.notch, g.hasNotch = *lk.notch, true
		}
		if lk.crack != nil {
			g.crack, g.hasCrack = *lk.crack, true
		}
		// Only meshing neighbours come close.
		for _, h := range gs[:i] {
			dist := math.Hypot(g.x-h.x, g.y-h.y)
			reach := pitch(g.n) + pitch(h.n)
			if math.Abs(dist-reach) > 1e-6 && dist <= reach+2*module+4 {
				log.Fatalf("gears of %d and %d teeth collide", g.n, h.n)
			}
		}
	}
	return gs
}

func num(v float64) string { return fmt.Sprintf("%.6g", v) }

func pt(deg, r float64) string {
	return fmt.Sprintf("%.1f %.1f", r*math.Cos(rad(deg)), r*math.Sin(rad(deg)))
}

// outline is one closed path round the teeth. A missing tooth leaves the
// root circle, a chipped one stops short and a notch bites into the rim.
func outline(g gear) string {
	r := pitch(g.n)
	root, tip, step := r-1.25*module, r+toothTip*module, 360/float64(g.n)
	var pts []string
	for k := range g.n {
		a := float64(k) * step
		off := mod(a-g.notch+180, 360) - 180
		switch {
		case g.hasNotch && math.Abs(off) < step*1.6:
			depth := root - 6 + math.Abs(off)/step*3
			pts = append(pts, pt(a-toothRoot*step, depth), pt(a+toothRoot*step, depth))
		case slices.Contains(g.missing, k):
			pts = append(pts, pt(a-toothRoot*step, root), pt(a, root), pt(a+toothRoot*step, root))
		default:
			t := tip
			if slices.Contains(g.chip, k) {
				t = r + .2*module
			}
			pts = append(pts, pt(a-toothRoot*step, root), pt(a-toothTop*step, t),
				pt(a+toothTop*step, t), pt(a+toothRoot*step, root))
		}
	}
	return "M" + strings.Join(pts, "L") + "Z"
}

func body(g gear) string {
	r := pitch(g.n)
	root := r - 1.25*module
	var b strings.Builder
	fmt.Fprintf(&b, `<path d="%s" fill="currentColor" fill-opacity=".18"/>`, outline(g))
	inner, hub := root-5, math.Max(5, r*.18)
	if r > 30 {
		fmt.Fprintf(&b, `<circle r="%s" stroke-opacity=".6"/>`, num(inner))
		var sp strings.Builder
		for i := range g.spokes {
			a := float64(i) * 360 / float64(g.spokes)
			fmt.Fprintf(&sp, "M%sL%s", pt(a, hub), pt(a, inner))
		}
		fmt.Fprintf(&b, `<path d="%s" stroke-opacity=".6"/>`, sp.String())
	}
	fmt.Fprintf(&b, `<circle r="%s"/><circle r="%s" fill="currentColor" fill-opacity=".4"/>`, num(hub), num(math.Max(2, hub*.4)))
	if g.hasCrack {
		var pts []string
		for i, f := range []float64{1, .8, .62, .45} {
			a := g.crack - 3
			if i%2 == 1 {
				a = g.crack + 4
			}
			pts = append(pts, pt(a, root*f))
		}
		fmt.Fprintf(&b, `<path d="M%s"/>`, strings.Join(pts, " L"))
	}
	return b.String()
}

// emit writes a train anchored at the given CSS position; class picks the
// Celestia (c) or Abyss (a) look.
func emit(b *strings.Builder, anchor, class string, gs []gear) {
	fmt.Fprintf(b, "<div class=\"gears\" style=\"%s\">\n", anchor)
	for _, g := range gs {
		ra := pitch(g.n) + module + 1
		c := class
		if g.ccw {
			c += " ccw"
		}
		fmt.Fprintf(b, `<svg class="gear %s" viewBox="%.1f %.1f %.1f %.1f" style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;animation-duration:%ss"><g transform="rotate(%.3f)">%s</g></svg>`+"\n",
			c, -ra, -ra, 2*ra, 2*ra, g.x-ra, g.y-ra, 2*ra, 2*ra, num(g.period), g.phase, body(g))
	}
	b.WriteString("</div>\n")
}

func deg(v float64) *float64 { return &v }

func main() {
	// Celestia, bottom right.
	celestia := train(56, 112, false,
		[]link{{0, 20, 185}, {0, 36, 262}, {2, 24, 195}, {1, 16, 235}, {3, 40, 255}},
		[]look{{spokes: 6}, {missing: []int{3, 4}}, {spokes: 4, notch: deg(40), crack: deg(200)},
			{chip: []int{7}}, {}, {missing: []int{30}}})
	// The Abyss, left middle.
	abyss := train(48, 96, true,
		[]link{{0, 18, -50}, {0, 30, 40}, {2, 40, 95}},
		[]look{{missing: []int{10}}, {chip: []int{0}}, {spokes: 3, crack: deg(60)},
			{spokes: 6, notch: deg(150), missing: []int{20, 21, 22}}})

	var b strings.Builder
	b.WriteString("{{/* Generated by gengears; do not edit. */}}\n{{define \"sky-gears\"}}")
	emit(&b, "right:30px;bottom:40px", "c", celestia)
	emit(&b, "left:-20px;top:44%", "a", abyss)
	b.WriteString("{{end}}\n")
	if err := os.WriteFile("templates/sky_gears.html", []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}
