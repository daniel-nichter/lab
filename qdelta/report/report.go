package report

import (
	"fmt"
	"math"
	"strconv"

	"github.com/daniel-nichter/lab/qdelta/delta"
	"github.com/daniel-nichter/lab/qdelta/slowlog"
)

const (
	HEADER_LINE_FMT = "#   %7s  %6s  %6s %5s %16s %s\n"
	DELTA_LINE_FMT  = "%-3d %7s  %6s  %6s %5s %16s %s\n"
)

func Print(deltas []delta.Metrics, iter ResultIter, minDelta float64) {
	fmt.Printf(HEADER_LINE_FMT, "-------", "------", "------", "-----", "----------------", "-----------")
	fmt.Printf(HEADER_LINE_FMT, "delta", "base", "comp", "obsrv", "ID", "fingerprint")
	fmt.Printf(HEADER_LINE_FMT, "-------", "------", "------", "-----", "----------------", "-----------")
	for i, d := range deltas {
		if iter.AbsDelta(d) < minDelta {
			return // don't print small deltas
		}
		fmt.Printf(DELTA_LINE_FMT,
			i+1,
			iter.Delta(d),
			iter.Base(d.Id),
			iter.Comp(d.Id),
			iter.Observed(d.Id),
			d.Id,
			iter.Fingerprint(d.Id),
		)
	}
}

type ResultIter interface {
	AbsDelta(delta.Metrics) float64
	Delta(delta.Metrics) string
	Base(id string) string
	Comp(id string) string
	Observed(id string) string
	Fingerprint(id string) string
}

type RealIter struct {
	metric  string
	base    slowlog.Result
	comp    slowlog.Result
	metrics map[string]delta.Result
}

func NewRealIter(metric string, base, comp slowlog.Result, metrics map[string]delta.Result) *RealIter {
	return &RealIter{
		metric:  metric,
		base:    base,
		comp:    comp,
		metrics: metrics,
	}
}

func (i *RealIter) AbsDelta(d delta.Metrics) float64 {
	switch i.metric {
	case "qps":
		return math.Abs(d.QPS)
	case "load":
		return math.Abs(d.Load)
	case "count":
		return math.Abs(d.CountPct) * 100
	case "exectime":
		return math.Abs(d.ExecTimePct) * 100
	}
	return 0
}

func (i *RealIter) Delta(d delta.Metrics) string {
	switch i.metric {
	case "qps":
		return ftoa(d.QPS, false)
	case "load":
		return ftoa(d.Load, false)
	case "count":
		return ftoa(d.CountPct, true)
	case "exectime":
		return ftoa(d.ExecTimePct, true)
	}
	return ""
}

func (i *RealIter) Base(id string) string {
	m, ok := i.metrics[id]
	if !ok {
		return "0"
	}
	return i.baseComp(m.Base)
}

func (i *RealIter) Comp(id string) string {
	m, ok := i.metrics[id]
	if !ok {
		return "0"
	}
	return i.baseComp(m.Comp)
}

func (i *RealIter) baseComp(m delta.Metrics) string {
	switch i.metric {
	case "qps":
		return ftoa(m.QPS, false)
	case "load":
		return ftoa(m.Load, false)
	case "count":
		return ftoa(m.CountPct, true)
	case "exectime":
		return ftoa(m.ExecTimePct, true)
	}
	return "?"
}

func (i *RealIter) Observed(id string) string {
	m := i.metrics[id]
	switch {
	case m.InBase && m.InComp:
		return "base"
	case m.InComp: // but not in base
		return "new"
	case m.InBase: // but not in comp
		return "miss"
	}
	return "?" // not in either?
}

func (i *RealIter) Fingerprint(id string) string {
	if _, ok := i.base.Class[id]; ok {
		return i.base.Class[id].Fingerprint
	}
	return i.comp.Class[id].Fingerprint
}

func ftoa(val float64, pct bool) string {
	if val == 0 {
		return "0"
	}
	if pct {
		val *= 100
	}
	if math.Abs(val) < 1 || pct { // < 1 or %
		return fmt.Sprintf("%.2f", val)
	}
	return strconv.Itoa(int(val)) // > 1  and not %
}
