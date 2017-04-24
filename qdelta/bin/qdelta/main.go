package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/daniel-nichter/lab/qdelta/delta"
	"github.com/daniel-nichter/lab/qdelta/report"
	"github.com/daniel-nichter/lab/qdelta/slowlog"
)

var (
	flagFile     string
	flagBase     string
	flagComp     string
	flagMinDelta float64
)

func init() {
	runtime.GOMAXPROCS(2)

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stderr)

	flag.StringVar(&flagFile, "file", "", "Slow log file")
	flag.StringVar(&flagBase, "base", "", "Baseline time range [since, until]")
	flag.StringVar(&flagComp, "comp", "", "Comparison time range [since, until]")
	flag.Float64Var(&flagMinDelta, "min-delta", 1, "Minimum delta")

	flag.Parse()

	// We don't accept any possitional arguments
	if len(flag.Args()) != 0 {
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	base, err := Parse(flagFile, flagBase)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("base duration: %s", base.End.Sub(base.Begin).String())

	comp, err := Parse(flagFile, flagComp)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("comp duration: %s", comp.End.Sub(comp.Begin).String())

	metrics := delta.Merge(base, comp)

	for _, orderBy := range []string{"qps", "load", "count", "exectime"} {
		deltas := delta.Delta(metrics, orderBy)
		iter := report.NewRealIter(orderBy, base, comp, metrics)

		fmt.Printf("# %s delta\n", orderBy)
		report.Print(deltas, iter, flagMinDelta)
		fmt.Println("")
	}
}

func Parse(file, timeRange string) (slowlog.Result, error) {
	var res slowlog.Result

	t := strings.Split(timeRange, "/")
	if len(t) != 2 {
		return res, fmt.Errorf("invalid time range: '%s': split returned %d timestamps, expected 2",
			timeRange, len(t))
	}
	since, err := time.Parse("2006-01-02T15:04:05", t[0])
	if err != nil {
		return res, fmt.Errorf("invalid timestamp: '%s': %s", t[0], err)
	}
	until, err := time.Parse("2006-01-02T15:04:05", t[1])
	if err != nil {
		return res, fmt.Errorf("invalid timestamp: '%s': %s", t[0], err)
	}

	i := slowlog.Interval{
		File:  flagFile,
		Since: since,
		Until: until,
	}
	p := slowlog.NewProcessor(time.Duration(0), 10)

	log.Printf("Processing %s since %s until %s...\n", file, since, until)
	res, err = p.Process(i)
	if err != nil {
		return res, err
	}
	return res, nil
}
