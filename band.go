package margaid

import (
	"fmt"
	"math"

	"github.com/erkkah/margaid/svg"
)

// defaultBandAlpha is the default fill opacity of band plots
const defaultBandAlpha = 0.35

// bandPoint is one valid, projected and clamped band position
type bandPoint struct {
	X  float64
	Lo float64
	Hi float64
}

// Band draws a semi-transparent confidence band between a lower and an
// upper bound series. The two series are expected to share the same X
// coordinates; they are walked in lockstep and the shorter one decides
// how many points are used.
//
// The band is drawn as one closed, filled region per contiguous run of
// valid points, so a mean line (or any other plot) can be overlaid by
// plotting it after the band. Both linear and logarithmic projections
// are supported.
//
// Point handling:
//   - Points where either bound or the shared X is non-finite (NaN or
//     +/-Inf) are treated as gaps: the fill is broken and resumed at
//     the next valid point. Insert e.g. MakeValue(x, math.NaN()) in a
//     bound series to mark a missing section.
//   - On logarithmic axes, points with values <= 0 cannot be drawn and
//     are treated as gaps as well. An axis range <= 0 on a log scale is
//     a configuration error and is reported like in other plots.
//   - If the bounds are inverted at a point (lower > upper) they are
//     swapped for that point, keeping the region simple (non
//     self-intersecting).
//   - Projected points are clamped to the plotting area, so out of
//     range bounds are cut off at the chart boundaries.
//   - Runs of fewer than two valid points produce no fill.
func (m *Margaid) Band(lower, upper *Series, using ...Using) {
	options := getPlotOptions(using)

	segments, err := m.getBandSegments(lower, upper, options.xAxis, options.yAxis)
	if err != nil {
		m.error(err.Error())
		return
	}

	id := m.addPlot(lower.title)
	if len(segments) == 0 {
		return
	}

	color := m.getPlotColor(id)
	m.g.
		Fill(color).
		FillOpacity(options.alpha).
		Stroke("none").
		Transform(
			svg.Translation(m.inset, m.height-m.inset),
			svg.Scaling(1, -1),
		)

	for _, segment := range segments {
		polygon := make([]struct{ X, Y float64 }, 0, 2*len(segment))
		for _, p := range segment {
			polygon = append(polygon, struct{ X, Y float64 }{p.X, p.Lo})
		}
		for i := len(segment) - 1; i >= 0; i-- {
			polygon = append(polygon, struct{ X, Y float64 }{segment[i].X, segment[i].Hi})
		}
		m.g.Polygon(polygon...)
	}
	m.g.FillOpacity(1).Transform()
}

// getBandSegments projects and clamps the bound series and splits them
// into contiguous runs of valid points.
func (m *Margaid) getBandSegments(lower, upper *Series, xAxis, yAxis Axis) ([][]bandPoint, error) {
	// Validate log ranges up front, reporting the same errors as
	// the other plot types.
	for _, axis := range []Axis{xAxis, yAxis} {
		if m.projections[axis] == Log {
			axisRange := m.ranges[axis]
			if axisRange.min <= 0 || axisRange.max <= 0 {
				return nil, fmt.Errorf("cannot have axis range <= 0 on log scale")
			}
		}
	}

	xLength := m.width - 2*m.inset
	yLength := m.height - 2*m.inset

	clamp := func(value, length float64) float64 {
		return math.Max(0, math.Min(length, value))
	}

	var segments [][]bandPoint
	var current []bandPoint

	closeRun := func() {
		// Runs of less than two points cannot enclose a region
		if len(current) >= 2 {
			segments = append(segments, current)
		}
		current = nil
	}

	lows := lower.Values()
	highs := upper.Values()

	for {
		lowNext := lows.Next()
		highNext := highs.Next()
		if !lowNext || !highNext {
			break
		}
		lo := lows.Get()
		hi := highs.Get()

		x := lo.X
		loY := lo.Y
		hiY := hi.Y

		valid := isFinite(x) && isFinite(loY) && isFinite(hiY)
		if valid && m.projections[xAxis] == Log && x <= 0 {
			valid = false
		}
		if valid && m.projections[yAxis] == Log && (loY <= 0 || hiY <= 0) {
			valid = false
		}

		if !valid {
			closeRun()
			continue
		}

		// Swap inverted bounds to keep the region simple
		if loY > hiY {
			loY, hiY = hiY, loY
		}

		px, err := m.project(x, xAxis)
		if err != nil {
			return nil, err
		}
		pLo, err := m.project(loY, yAxis)
		if err != nil {
			return nil, err
		}
		pHi, err := m.project(hiY, yAxis)
		if err != nil {
			return nil, err
		}

		current = append(current, bandPoint{
			X:  clamp(px, xLength),
			Lo: clamp(pLo, yLength),
			Hi: clamp(pHi, yLength),
		})
	}
	closeRun()

	return segments, nil
}
