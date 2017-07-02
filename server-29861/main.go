package main

// Debug https://jira.mongodb.org/browse/SERVER-29861
//
// Source: https://github.com/daniel-nichter/lab

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	DEFAULT_DB           = "test"
	DEFAULT_C            = "test"
	DEFAULT_N            = 100000
	DEFAULT_ITER         = 1
	DEFAULT_WAIT         = 0
	DEFAULT_SAFE_W       = 2
	DEFAULT_SAFE_WMODE   = ""
	DEFAULT_SAFE_TIMEOUT = 1000
	DEFAULT_SAFE_FSYNC   = false
	DEFAULT_SAFE_J       = true
)

var (
	flagDb           string
	flagC            string
	flagDocs         uint
	flagTests        uint
	flagWait         uint
	flagSafeW        int
	flagSafeWMode    string
	flagSafeWTimeout int
	flagSafeFSync    bool
	flagSafeJ        bool
)

func init() {
	flag.StringVar(&flagDb, "db", DEFAULT_DB, "Database")
	flag.StringVar(&flagC, "c", DEFAULT_C, "Collection")
	flag.UintVar(&flagDocs, "docs", DEFAULT_N, "Number of docs to insert")
	flag.UintVar(&flagTests, "tests", DEFAULT_ITER, "Number of tests to run")
	flag.UintVar(&flagWait, "wait", DEFAULT_WAIT, "Wait time (ms) before rs.stepDown (0 = random)")
	flag.IntVar(&flagSafeW, "safe-w", DEFAULT_SAFE_W, "Safe.W")
	flag.StringVar(&flagSafeWMode, "safe-wmode", DEFAULT_SAFE_WMODE, "Safe.WMode")
	flag.IntVar(&flagSafeWTimeout, "safe-timeout", DEFAULT_SAFE_TIMEOUT, "Safe.Timeout (milliseconds)")
	flag.BoolVar(&flagSafeFSync, "safe-fsync", DEFAULT_SAFE_FSYNC, "Safe.FSync")
	flag.BoolVar(&flagSafeJ, "safe-j", DEFAULT_SAFE_J, "Safe.J")

	rand.Seed(28395)

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds)
	log.SetOutput(os.Stderr)
}

type conn struct {
	url   string
	db    string
	c     string
	safe  *mgo.Safe
	s     *mgo.Session
	C     *mgo.Collection
	nchan chan uint // number of reported (not actual) docs written
}

var docs []interface{}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Printf("Usage: %s [flags] URL\n", os.Args[0])
		os.Exit(1)
	}

	// Make all the docs once. Doesn't matter what we insert, so we can reuse
	// the same docs rather than waste time re-making them.
	docs = make([]interface{}, flagDocs)
	for i := range docs {
		e := map[string]interface{}{
			"_id": bson.NewObjectId().String(),
			"n":   i,
		}
		docs[i] = e
	}
	log.Printf("%d docs to insert", len(docs))

	// Connection info doesn't change, but we'll need a new connection for
	// every test. So this little factory makes *conn that connect() can turn
	// into new connections with mgo.Session and mgo.Collection ready to use.
	safe := &mgo.Safe{
		W:        flagSafeW,
		WMode:    flagSafeWMode,
		WTimeout: flagSafeWTimeout,
		FSync:    flagSafeFSync,
		J:        flagSafeJ,
	}
	log.Printf("write concern: %#v\n", safe)
	log.Printf("url: %s\n", args[0])
	newConn := func() (*conn, error) {
		c := &conn{
			url:   args[0],
			db:    flagDb,
			c:     flagC,
			safe:  safe,
			nchan: make(chan uint, 1),
		}
		var err error
		c.s, err = mgo.Dial(c.url)
		if err != nil {
			return nil, fmt.Errorf("mgo.Dial: %s", err)
		}
		c.s.SetSafe(c.safe) // set write concern
		c.C = c.s.DB(c.db).C(c.c)
		return c, nil
	}

	// //////////////////////////////////////////////////////////////////////
	// Test loop
	// //////////////////////////////////////////////////////////////////////
	testNo := uint(0)
	for testNo < flagTests {
		testNo++
		log.Printf("test %d of %d\n", testNo, flagTests)

		var wait time.Duration
		if flagWait == 0 {
			// random wait
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			i := 500 + r.Intn(2000)
			wait = time.Duration(i) * time.Millisecond
			log.Printf("wait: %s", wait)
		} else {
			wait = time.Duration(flagWait) * time.Millisecond
		}

		// New connections for insert and rs.stepDown
		conn1, err := newConn()
		if err != nil {
			log.Fatal(err)
		}
		conn2, err := newConn()
		if err != nil {
			log.Fatal(err)
		}

		// Start insert
		go conn1.insert()

		// Wait for it...
		log.Printf("wait")
		time.Sleep(wait)

		// Step down the primary
		log.Printf("rs.stepDown")
		conn2.stepDown()

		// Wait up to 30s for rs to recover
		log.Printf("rs recovering")
		var conn3 *conn
		timeout := time.After(30 * time.Second)
		for {
			time.Sleep(2 * time.Second) // rs takes a few seconds to recover

			var err error
			conn3, err = newConn()
			if err == nil {
				log.Println("rs recovered")
				break
			}

			select {
			case <-timeout:
				log.Fatal("Timeout waiting for rs to recover")
			default:
			}
		}

		// Wait for insert to return n docs reported written
		nReported := <-conn1.nchan

		// Remove all docs to see how many were actually written
		nActual, err := conn3.remove()
		if err != nil {
			log.Fatal(err)
		}

		// Log the results
		ok := nReported == nActual
		fmt.Printf("%d,%d,%t\n", nReported, nActual, ok)

		// Clean up test and repeat
		conn1.s.Close()
		conn2.s.Close()
		conn3.s.Close()
	}
}

func (c *conn) insert() {
	log.Printf("inserting...")
	err := c.C.Insert(docs...)
	if err != nil {
		lerr := err.(*mgo.LastError)
		c.nchan <- uint(lerr.N)
	}
	c.nchan <- uint(len(docs))
}

func (c *conn) stepDown() {
	var res bson.D
	c.s.Run(bson.D{{"replSetStepDown", 10}, {"secondaryCatchUpPeriodSecs", 5}}, res)
}

func (c *conn) remove() (uint, error) {
	// 10s to remove all docs
	c.safe.WTimeout = 10000
	c.s.SetSafe(c.safe)

	log.Println("removing")
	w, err := c.C.RemoveAll(bson.D{})
	if err != nil {
		return 0, err
	}
	return uint(w.Removed), nil
}
