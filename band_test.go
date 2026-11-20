package margaid

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/erkkah/margaid/xt"
)

// renderToString renders a diagram to a string for inspection
func renderToString(m *Margaid) string {
	var builder strings.Builder
	m.Render(&builder)
	return builder.String()
}

// bandDiagram creates a 200x200 diagram with a 72x72 plotting area
// (default inset 64) and fixed ranges
func bandDiagram(options ...Option) *Margaid {
	base := []Option{
		WithRange(XAxis, 0, 4),
		WithRange(YAxis, 0, 5),
	}
	return New(200, 200, append(base, options...)...)
}

// gappedBounds returns lower and upper bound series with a NaN gap at x=2
func gappedBounds() (lower, upper *Series) {
	nan := math.NaN()
	lower = NewSeries()
	lower.Add(
		MakeValue(0, 1),
		MakeValue(1, 2),
		MakeValue(2, nan),
		MakeValue(3, 2),
		MakeValue(4, 1),
	)
	upper = NewSeries()
	upper.Add(
		MakeValue(0, 3),
		MakeValue(1, 4),
		MakeValue(2, nan),
		MakeValue(3, 4),
		MakeValue(4, 3),
	)
	return
}

func TestBandBreaksAtGap(t *testing.T) {
	x := xt.X(t)

	lower, upper := gappedBounds()
	diagram := bandDiagram()
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)

	// The gap at x=2 splits the band into two closed subpaths
	x.True(strings.Contains(rendered,
		`d="M0,1.440000e+01 L18,2.880000e+01 L18,5.760000e+01 L0,4.320000e+01 Z "`),
		"First segment missing or wrong: "+rendered)
	x.True(strings.Contains(rendered,
		`d="M54,2.880000e+01 L72,1.440000e+01 L72,4.320000e+01 L54,5.760000e+01 Z "`),
		"Second segment missing or wrong: "+rendered)

	// Semi-transparent fill with the default alpha
	x.True(strings.Contains(rendered, `fill-opacity="0.35"`),
		"Default fill opacity missing: "+rendered)

	// Gap markers must never leak into the output
	x.False(strings.Contains(rendered, "NaN"), "NaN leaked into output")
}

func TestBandLogProjection(t *testing.T) {
	x := xt.X(t)

	// y values crossing a non-positive section, which cannot be
	// drawn on a log scale and must break the band
	lower := NewSeries()
	lower.Add(
		MakeValue(0, 2),
		MakeValue(1, 50),
		MakeValue(2, -5),
		MakeValue(3, 50),
		MakeValue(4, 2),
	)
	upper := NewSeries()
	upper.Add(
		MakeValue(0, 10),
		MakeValue(1, 100),
		MakeValue(2, 0),
		MakeValue(3, 100),
		MakeValue(4, 10),
	)

	diagram := bandDiagram(
		WithRange(YAxis, 1, 100),
		WithProjection(YAxis, Log),
	)
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)

	// log10 projected onto [0..72]: y -> log10(y)/log10(100) * 72
	project := func(v float64) string {
		p := math.Log10(v) / 2 * 72
		if float64(int(p)) == p {
			return fmt.Sprintf("%d", int(p))
		}
		return fmt.Sprintf("%e", p)
	}

	// Two segments around the non-positive gap at x=2
	first := fmt.Sprintf(`d="M0,%s L18,%s L18,%s L0,%s Z "`,
		project(2), project(50), project(100), project(10))
	second := fmt.Sprintf(`d="M54,%s L72,%s L72,%s L54,%s Z "`,
		project(50), project(2), project(10), project(100))

	x.True(strings.Contains(rendered, first),
		"First log segment missing or wrong: "+rendered)
	x.True(strings.Contains(rendered, second),
		"Second log segment missing or wrong: "+rendered)
	x.False(strings.Contains(rendered, "NaN"), "NaN leaked into output")
	x.False(strings.Contains(rendered, "-Inf"), "-Inf leaked into output")
}

func TestBandInvertedBoundsAreSwapped(t *testing.T) {
	x := xt.X(t)

	// Bounds cross between x=0 and x=2
	lower := NewSeries()
	lower.Add(MakeValue(0, 4), MakeValue(2, 1))
	upper := NewSeries()
	upper.Add(MakeValue(0, 1), MakeValue(2, 4))

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)

	// Swapping per point yields a simple rectangle, not a bowtie
	x.True(strings.Contains(rendered,
		`d="M0,1.440000e+01 L36,1.440000e+01 L36,5.760000e+01 L0,5.760000e+01 Z "`),
		"Swapped band missing or wrong: "+rendered)
}

func TestBandClampsToPlotArea(t *testing.T) {
	x := xt.X(t)

	// Bounds outside the axis ranges on both axes
	lower := NewSeries()
	lower.Add(MakeValue(0, -3), MakeValue(10, 1))
	upper := NewSeries()
	upper.Add(MakeValue(0, 100), MakeValue(10, 6))

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)

	// Everything is clamped to the 72x72 plotting area
	x.True(strings.Contains(rendered,
		`d="M0,0 L72,1.440000e+01 L72,72 L0,72 Z "`),
		"Clamped band missing or wrong: "+rendered)
}

func TestBandDifferentLengths(t *testing.T) {
	x := xt.X(t)

	lower := NewSeries()
	lower.Add(MakeValue(0, 1), MakeValue(2, 2), MakeValue(4, 1))
	upper := NewSeries()
	upper.Add(
		MakeValue(0, 3),
		MakeValue(2, 4),
		MakeValue(4, 3),
		MakeValue(5, 3),
		MakeValue(6, 3),
	)

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)

	// The shorter series decides: three shared x positions
	x.True(strings.Contains(rendered,
		`d="M0,1.440000e+01 L36,2.880000e+01 L72,1.440000e+01 L72,4.320000e+01 L36,5.760000e+01 L0,4.320000e+01 Z "`),
		"Band missing or wrong: "+rendered)
}

func TestBandEmptyIntervals(t *testing.T) {
	x := xt.X(t)

	nan := math.NaN()

	// All points missing: nothing is drawn
	lower := NewSeries()
	lower.Add(MakeValue(0, nan), MakeValue(1, nan))
	upper := NewSeries()
	upper.Add(MakeValue(0, nan), MakeValue(1, nan))

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)
	x.False(strings.Contains(rendered, "<path"), "Empty band should not draw: "+rendered)

	// A single valid point cannot enclose a region either
	lower = NewSeries()
	lower.Add(MakeValue(0, nan), MakeValue(1, 1), MakeValue(2, nan))
	upper = NewSeries()
	upper.Add(MakeValue(0, nan), MakeValue(1, 2), MakeValue(2, nan))

	diagram = bandDiagram()
	diagram.Band(lower, upper)
	rendered = renderToString(diagram)
	x.False(strings.Contains(rendered, "<path"), "Single point band should not draw: "+rendered)

	// Empty series are accepted silently
	diagram = bandDiagram()
	diagram.Band(NewSeries(), NewSeries())
	rendered = renderToString(diagram)
	x.False(strings.Contains(rendered, "<path"), "Empty series should not draw: "+rendered)
}

func TestBandAlphaOption(t *testing.T) {
	x := xt.X(t)

	lower, upper := gappedBounds()
	diagram := bandDiagram()
	diagram.Band(lower, upper, UsingAlpha(0.5))
	rendered := renderToString(diagram)
	x.True(strings.Contains(rendered, `fill-opacity="0.5"`),
		"Custom alpha missing: "+rendered)

	// Out of range alpha is clamped to 1, which is the default
	// and leaves no fill-opacity attribute behind
	lower, upper = gappedBounds()
	diagram = bandDiagram()
	diagram.Band(lower, upper, UsingAlpha(7))
	rendered = renderToString(diagram)
	x.False(strings.Contains(rendered, "fill-opacity"),
		"Fully opaque band should not set fill-opacity: "+rendered)
}

func TestBandDoesNotLeakOpacity(t *testing.T) {
	x := xt.X(t)

	lower, upper := gappedBounds()
	mean := NewSeries()
	mean.Add(MakeValue(0, 2), MakeValue(4, 2))

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	diagram.Line(mean)
	diagram.Frame()
	diagram.Title("title")
	rendered := renderToString(diagram)

	// The band opacity must not bleed into later plots, axes or text:
	// exactly one group carries the fill-opacity attribute
	x.Equal(strings.Count(rendered, "fill-opacity"), 1)
}

func TestBandInvalidLogRange(t *testing.T) {
	x := xt.X(t)

	lower, upper := gappedBounds()
	diagram := bandDiagram(
		WithRange(YAxis, -1, 100),
		WithProjection(YAxis, Log),
	)
	diagram.Band(lower, upper)
	rendered := renderToString(diagram)
	x.True(strings.Contains(rendered, "cannot have axis range"),
		"Expected log range error: "+rendered)
}

func TestBandOverlaidWithLine(t *testing.T) {
	x := xt.X(t)

	lower, upper := gappedBounds()

	mean := NewSeries()
	mean.Add(
		MakeValue(0, 2),
		MakeValue(1, 3),
		MakeValue(2, 3),
		MakeValue(3, 3),
		MakeValue(4, 2),
	)

	diagram := bandDiagram()
	diagram.Band(lower, upper)
	diagram.Line(mean)
	rendered := renderToString(diagram)

	// The mean line is drawn on top, as its own path element,
	// unaffected by the band fill
	x.True(strings.Contains(rendered,
		`d="M0,2.880000e+01 L18,4.320000e+01 L36,4.320000e+01 L54,4.320000e+01 L72,2.880000e+01 "`),
		"Mean line missing or wrong: "+rendered)

	// Band polygons and the line stay separate path elements
	x.Equal(strings.Count(rendered, "<path"), 3)
}

func TestLinePlotUnchanged(t *testing.T) {
	x := xt.X(t)

	series := NewSeries()
	series.Add(MakeValue(0, 0), MakeValue(4, 5))

	diagram := bandDiagram()
	diagram.Line(series)
	rendered := renderToString(diagram)

	x.True(strings.Contains(rendered, `d="M0,0 L72,72 "`),
		"Line plot changed: "+rendered)
	x.False(strings.Contains(rendered, "fill-opacity"),
		"Line plot should not set fill opacity: "+rendered)
}

func TestNonFiniteValuesIgnoredInRange(t *testing.T) {
	x := xt.X(t)

	nan := math.NaN()
	series := NewSeries()
	series.Add(
		MakeValue(0, 1),
		MakeValue(1, nan),
		MakeValue(2, math.Inf(1)),
		MakeValue(3, 4),
	)

	x.Equal(series.MinY(), 1.0)
	x.Equal(series.MaxY(), 4.0)
	x.Equal(series.MinX(), 0.0)
	x.Equal(series.MaxX(), 3.0)

	// Autorange over gapped band bounds stays finite
	lower, upper := gappedBounds()
	diagram := New(200, 200,
		WithAutorange(XAxis, lower),
		WithAutorange(YAxis, lower, upper),
	)
	x.Equal(diagram.ranges[YAxis].min, 1.0)
	x.Equal(diagram.ranges[YAxis].max, 4.0)
}
