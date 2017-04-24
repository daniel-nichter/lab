package report_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/daniel-nichter/lab/qdelta/delta"
	"github.com/daniel-nichter/lab/qdelta/report"
	"github.com/daniel-nichter/lab/qdelta/slowlog"
	"github.com/go-test/deep"
)

func loadSlowlogResults(file string) (slowlog.Result, error) {
	var res slowlog.Result
	file = filepath.Join("../test/results", file)
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

func loadMergedResults(file string) (map[string]delta.Result, error) {
	var res map[string]delta.Result
	file = filepath.Join("../test/results", file)
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

func dump(v interface{}) {
	bytes, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println("dump: ", string(bytes))
}

func Test001(t *testing.T) {
	base, err := loadSlowlogResults("001-base.json")
	if err != nil {
		t.Fatal(err)
	}
	comp, err := loadSlowlogResults("001-comp.json")
	if err != nil {
		t.Fatal(err)
	}

	metrics := delta.Merge(base, comp)

	/////////////////////////////////////////////////////////////////////////
	// QPS

	deltas := delta.Delta(metrics, "qps")
	iter := report.NewRealIter("qps", base, comp, metrics)

	// [delta,base,comp,obsrv]
	expect := [][]string{
		{"100", "0", "100", "new"}, // D
		{"100", "0", "100", "new"}, // E
		{"0", "55", "55", "base"},  // A
		{"0", "38", "38", "base"},  // B
		{"0", "5", "5", "base"},    // C
	}
	for i, d := range deltas {
		got := []string{"", "", "", ""}
		got[0] = iter.Delta(d)
		got[1] = iter.Base(d.Id)
		got[2] = iter.Comp(d.Id)
		got[3] = iter.Observed(d.Id)
		if diff := deep.Equal(got, expect[i]); diff != nil {
			for _, d := range diff {
				t.Errorf("qps %d: %s", i, d)
			}
		}
	}

	/////////////////////////////////////////////////////////////////////////
	// Count %

	deltas = delta.Delta(metrics, "count")
	iter = report.NewRealIter("count", base, comp, metrics)

	// [delta,base,comp,obsrv]
	expect = [][]string{
		{"-37.04", "55.56", "18.52", "base"}, // A
		{"33.33", "0", "33.33", "new"},       // D
		{"33.33", "0", "33.33", "new"},       // E
		{"-25.93", "38.89", "12.96", "base"}, // B
		{"-3.70", "5.56", "1.85", "base"},    // C
	}
	for i, d := range deltas {
		got := []string{"", "", "", ""}
		got[0] = iter.Delta(d)
		got[1] = iter.Base(d.Id)
		got[2] = iter.Comp(d.Id)
		got[3] = iter.Observed(d.Id)
		if diff := deep.Equal(got, expect[i]); diff != nil {
			for _, d := range diff {
				t.Errorf("count %d: %s", i, d)
			}
		}
	}
}
