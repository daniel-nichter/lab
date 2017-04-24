package delta_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/daniel-nichter/lab/qdelta/delta"
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

	expectRes, err := loadMergedResults("001-merged.json")
	if err != nil {
		t.Fatal(err)
	}
	gotRes := delta.Merge(base, comp)

	if diff := deep.Equal(gotRes, expectRes); diff != nil {
		for _, d := range diff {
			t.Error(d)
		}
	}

	gotDeltas := delta.Delta(gotRes, "qps")
	expectDeltas := []delta.Metrics{
		{
			Id:          "D",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "E",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "A",
			QPS:         0,
			Load:        0,
			CountPct:    -0.3703703703703704,
			ExecTimePct: -0.12323943661971831,
		},
		{
			Id:          "B",
			QPS:         0,
			Load:        0,
			CountPct:    -0.2592592592592593,
			ExecTimePct: -0.24647887323943662,
		},
		{
			Id:          "C",
			QPS:         0,
			Load:        0,
			CountPct:    -0.037037037037037035,
			ExecTimePct: -0.12323943661971831,
		},
	}
	if diff := deep.Equal(gotDeltas, expectDeltas); diff != nil {
		for _, d := range diff {
			t.Error(d)
		}
	}

	gotDeltas = delta.Delta(gotRes, "count")
	expectDeltas = []delta.Metrics{
		{
			Id:          "A",
			QPS:         0,
			Load:        0,
			CountPct:    -0.3703703703703704,
			ExecTimePct: -0.12323943661971831,
		},
		{
			Id:          "D",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "E",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "B",
			QPS:         0,
			Load:        0,
			CountPct:    -0.2592592592592593,
			ExecTimePct: -0.24647887323943662,
		},
		{
			Id:          "C",
			QPS:         0,
			Load:        0,
			CountPct:    -0.037037037037037035,
			ExecTimePct: -0.12323943661971831,
		},
	}
	if diff := deep.Equal(gotDeltas, expectDeltas); diff != nil {
		for _, d := range diff {
			t.Error(d)
		}
	}

	gotDeltas = delta.Delta(gotRes, "exectime")
	expectDeltas = []delta.Metrics{
		{
			Id:          "B",
			QPS:         0,
			Load:        0,
			CountPct:    -0.2592592592592593,
			ExecTimePct: -0.24647887323943662,
		},
		{
			Id:          "D",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "E",
			QPS:         100,
			Load:        1.9444444444444444,
			CountPct:    0.3333333333333333,
			ExecTimePct: 0.24647887323943662,
		},
		{
			Id:          "A",
			QPS:         0,
			Load:        0,
			CountPct:    -0.3703703703703704,
			ExecTimePct: -0.12323943661971831,
		},
		{
			Id:          "C",
			QPS:         0,
			Load:        0,
			CountPct:    -0.037037037037037035,
			ExecTimePct: -0.12323943661971831,
		},
	}
	if diff := deep.Equal(gotDeltas, expectDeltas); diff != nil {
		for _, d := range diff {
			t.Error(d)
		}
	}
}
