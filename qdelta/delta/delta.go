package delta

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/daniel-nichter/lab/qdelta/slowlog"
)

type Metrics struct {
	Id          string
	QPS         float64
	Load        float64
	CountPct    float64
	ExecTimePct float64
}

type Result struct {
	InBase bool
	InComp bool
	Base   Metrics
	Comp   Metrics
}

func Merge(base, comp slowlog.Result) map[string]Result {
	metrics := map[string]Result{}

	gTotalTime := base.End.Sub(base.Begin).Seconds()
	gTotalQueries := float64(base.Global.TotalQueries)
	gTotalExecTime := base.Global.Metrics.TimeMetrics["Query_time"].Sum

	for id, class := range base.Class {
		d := Metrics{
			QPS:         float64(class.TotalQueries) / gTotalTime,
			Load:        class.Metrics.TimeMetrics["Query_time"].Sum / gTotalTime,
			CountPct:    float64(class.TotalQueries) / gTotalQueries,
			ExecTimePct: class.Metrics.TimeMetrics["Query_time"].Sum / gTotalExecTime,
		}
		metrics[id] = Result{
			InBase: true,
			Base:   d,
		}
	}

	gTotalTime = comp.End.Sub(comp.Begin).Seconds()
	gTotalQueries = float64(comp.Global.TotalQueries)
	gTotalExecTime = comp.Global.Metrics.TimeMetrics["Query_time"].Sum

	for id, class := range comp.Class {
		d := Metrics{
			QPS:         float64(class.TotalQueries) / gTotalTime,
			Load:        class.Metrics.TimeMetrics["Query_time"].Sum / gTotalTime,
			CountPct:    float64(class.TotalQueries) / gTotalQueries,
			ExecTimePct: class.Metrics.TimeMetrics["Query_time"].Sum / gTotalExecTime,
		}
		if r, ok := metrics[id]; !ok {
			// new query
			metrics[id] = Result{
				InComp: true,
				Comp:   d,
			}
		} else {
			// same query seen again
			r.InComp = true
			r.Comp = d
			metrics[id] = r
		}
	}

	return metrics
}

func Delta(metrics map[string]Result, orderBy string) []Metrics {
	deltas := make([]Metrics, len(metrics))
	i := 0
	for id, r := range metrics {
		delta := metricsDelta(r.Base, r.Comp)
		delta.Id = id
		deltas[i] = delta
		i++
	}
	switch orderBy {
	case "qps":
		sort.Sort(byQPS(deltas))
	case "load":
		sort.Sort(byLoad(deltas))
	case "count":
		sort.Sort(byCountPct(deltas))
	case "exectime":
		sort.Sort(byExecTimePct(deltas))
	default:
		panic(fmt.Sprintf("invalid orderBy: %s", orderBy))
	}
	return deltas
}

func metricsDelta(base, comp Metrics) Metrics {
	return Metrics{
		QPS:         diff(base.QPS, comp.QPS),
		Load:        diff(base.Load, comp.Load),
		CountPct:    diff(base.CountPct, comp.CountPct),
		ExecTimePct: diff(base.ExecTimePct, comp.ExecTimePct),
	}
}

func diff(a, b float64) float64 {
	// 20 -> 40 == 20 - 40 = -20 * -1 =  20 (increase)
	// 40 -> 20 == 40 - 20 =  20 * -1 = -20 (decrease)
	//  0 -> 40 ==  0 - 40 = -40 * -1 =  40 (increase)
	if a == b {
		return 0
	}
	return (a - b) * -1.0
}

// //////////////////////////////////////////////////////////////////////////
// Sort funcs
// //////////////////////////////////////////////////////////////////////////

// All sorts are descending to put the biggest absolute difference at top.

type byQPS []Metrics

func (a byQPS) Len() int      { return len(a) }
func (a byQPS) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byQPS) Less(i, j int) bool {
	if a[i].QPS == a[j].QPS {
		// Sort by Id to make tests deterministic
		return strings.Compare(a[i].Id, a[j].Id) < 0
	}
	return math.Abs(a[i].QPS) > math.Abs(a[j].QPS)
}

type byLoad []Metrics

func (a byLoad) Len() int      { return len(a) }
func (a byLoad) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byLoad) Less(i, j int) bool {
	if a[i].Load == a[j].Load {
		// Sort by Id to make tests deterministic
		return strings.Compare(a[i].Id, a[j].Id) < 0
	}
	return math.Abs(a[i].Load) > math.Abs(a[j].Load)
}

type byCountPct []Metrics

func (a byCountPct) Len() int      { return len(a) }
func (a byCountPct) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byCountPct) Less(i, j int) bool {
	if a[i].CountPct == a[j].CountPct {
		// Sort by Id to make tests deterministic
		return strings.Compare(a[i].Id, a[j].Id) < 0
	}
	return math.Abs(a[i].CountPct) > math.Abs(a[j].CountPct)
}

type byExecTimePct []Metrics

func (a byExecTimePct) Len() int      { return len(a) }
func (a byExecTimePct) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byExecTimePct) Less(i, j int) bool {
	if a[i].ExecTimePct == a[j].ExecTimePct {
		// Sort by Id to make tests deterministic
		return strings.Compare(a[i].Id, a[j].Id) < 0
	}
	return math.Abs(a[i].ExecTimePct) > math.Abs(a[j].ExecTimePct)
}
