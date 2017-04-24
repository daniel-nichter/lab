package slowlog

import (
	"log"
	"os"
	"time"

	"github.com/go-mysql/query"
	"github.com/go-mysql/slowlog"
)

const SLOWLOG_TS_FORMAT = "060102 15:04:05" // YYMMDD

type Interval struct {
	File  string
	Since time.Time
	Until time.Time
}

type Result struct {
	Begin time.Time // actual vs. Since, used to calc QPS
	End   time.Time // actual vs. Until, used to calc QPS
	slowlog.Result
}

type Processor struct {
	utcOffset   time.Duration // UTC offset in hours for the system time zone
	outlierTime float64       // @@global.slow_query_log_always_write_time
}

func NewProcessor(utcOffset time.Duration, outlierTime float64) *Processor {
	return &Processor{
		utcOffset:   utcOffset,
		outlierTime: outlierTime,
	}
}

func (p *Processor) Process(i Interval) (Result, error) {
	res := Result{}

	file, err := os.Open(i.File)
	if err != nil {
		return res, err
	}
	defer file.Close() // don't leak fd

	// Run fingerprinter in goroutine in case it crashes.
	queryChan := make(chan string, 1)
	fingerprintChan := make(chan string, 1)
	crashChan := make(chan interface{}, 1)
	go p.fingerprinter(queryChan, fingerprintChan, crashChan)
	defer close(queryChan) // stop that ^ goroutine

	// Use aggregator to group events by fingerprint and calculate stats.
	a := slowlog.NewAggregator(true, p.utcOffset, p.outlierTime)

	// Run slow log parser, recv events from its EventChan().
	slp := slowlog.NewFileParser(file)
	if err := slp.Start(slowlog.Options{}); err != nil {
		return res, err
	}
	defer slp.Stop()

	var (
		firstEvent *slowlog.Event
		lastEvent  *slowlog.Event
	)

	for event := range slp.Events() {
		var ts time.Time
		var err error
		if event.Ts == "" {
			if firstEvent == nil {
				continue // keep looking for known start ts
			}

			// At some time after since, so presume this event which has no ts
			// is also before until
		} else {
			// Filter out events not in [since, until)
			ts, err = time.Parse(SLOWLOG_TS_FORMAT, event.Ts)
			if err != nil {
				log.Printf("invalid slow log timestamp (recovering): %s: %s", event.Ts, err)
				continue
			}
			if ts.Before(i.Since) {
				continue
			}
			if ts.After(i.Until) {
				if lastEvent != nil && lastEvent.Ts != "" {
					// Can ignore err here because we already parsed event ts ^
					ts, _ := time.Parse(SLOWLOG_TS_FORMAT, lastEvent.Ts)
					res.End = ts
				} else {
					ts, err := time.Parse(SLOWLOG_TS_FORMAT, event.Ts)
					if err != nil {
						log.Printf("invalid slow log timestamp (recovering): %s: %s", event.Ts, err)
						ts = i.Until
					}
					res.End = ts
				}
				log.Printf("last event at %s", res.End)
				break
			}
		}

		// Event is in [since, until), fingerprint and save it
		queryChan <- event.Query
		select {
		case fingerprint := <-fingerprintChan:
			id := query.Id(fingerprint)
			a.AddEvent(event, id, fingerprint)
		case err := <-crashChan:
			log.Printf("fingerprinter crashed (recovering): %s: %s", err, event.Query)
			go p.fingerprinter(queryChan, fingerprintChan, crashChan)
		}

		// Save first and last events so we can determine actual begin and end
		// times of the interval. Slow log isn't guaranteed to have the full
		// time range ([since, until)).
		if firstEvent == nil {
			log.Printf("first event at %s", ts)
			firstEvent = &event
			res.Begin = ts
		}
		lastEvent = &event
	}

	// Calculate global and class metric stats, get final results.
	res.Result = a.Finalize()
	return res, nil
}

func (p *Processor) fingerprinter(in, out chan string, crash chan interface{}) {
	defer func() {
		if err := recover(); err != nil {
			crash <- err
		}
	}()
	for q := range in {
		out <- query.Fingerprint(q)
	}
}
