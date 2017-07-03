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
	"strings"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	DEFAULT_DB            = "test"
	DEFAULT_C             = "test"
	DEFAULT_N             = 250000
	DEFAULT_ITER          = 1
	DEFAULT_WAIT          = 0
	DEFAULT_SAFE_W        = 2
	DEFAULT_SAFE_WMODE    = ""
	DEFAULT_SAFE_TIMEOUT  = 1000
	DEFAULT_SAFE_FSYNC    = false
	DEFAULT_SAFE_J        = true
	DEFAULT_STEPDOWN_TIME = 10
)

var (
	flagUsername     string
	flagPassword     string
	flagAuthDb       string
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
	flag.StringVar(&flagUsername, "username", "", "Username")
	flag.StringVar(&flagPassword, "password", "", "Password")
	flag.StringVar(&flagAuthDb, "auth-db", "", "Auth db")
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

type writeResult struct {
	n   uint
	err string
}

type conn struct {
	url  string
	db   string
	c    string
	safe *mgo.Safe
	s    *mgo.Session
	C    *mgo.Collection
	wr   chan writeResult
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
	var cred *mgo.Credential
	if flagUsername != "" && flagPassword != "" {
		cred = &mgo.Credential{
			Username:  flagUsername,
			Password:  flagPassword,
			Source:    flagAuthDb,
			Mechanism: "SCRAM-SHA-1",
		}
		log.Printf("login: %#v", *cred)
	}

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
			url:  args[0],
			db:   flagDb,
			c:    flagC,
			safe: safe,
			wr:   make(chan writeResult, 1),
		}
		var err error
		c.s, err = mgo.Dial(c.url)
		if err != nil {
			return nil, fmt.Errorf("mgo.Dial: %s", err)
		}
		if cred != nil {
			if err := c.s.Login(cred); err != nil {
				log.Fatal(err)
			}
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

		// First make sure the lab is clean: 0 docs in the collection
		n, err := conn1.remove()
		if err != nil {
			log.Fatal(err)
		}
		if n != 0 {
			log.Fatalf("%s.%s has %d docs at start of test %d, expected 0",
				conn1.db, conn1.c, n, testNo)
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
		time.Sleep(time.Duration(DEFAULT_STEPDOWN_TIME) * time.Second)
		var conn3 *conn
		timeout := time.After(10 * time.Second)
		for {
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

			time.Sleep(1 * time.Second) // rs takes a few seconds to recover
		}

		// Wait for insert to return n docs reported written
		wr := <-conn1.wr

		// Remove all docs to see how many were actually written
		nActual, err := conn3.remove()
		if err != nil {
			log.Fatal(err)
		}

		// Log the results
		ok := wr.n == nActual

		errStr := ""
		if strings.Contains(wr.err, "operation was interrupted") {
			errStr = "int"
		} else if strings.Contains(wr.err, "EOF") {
			errStr = "eof"
		} else {
			errStr = "aok"
		}
		fmt.Printf("%d,%d,%.3f,%s,%t\n", wr.n, nActual, wait.Seconds(), errStr, ok)

		// Clean up test and repeat
		conn1.s.Close()
		conn2.s.Close()
		conn3.s.Close()
	}
}

func (c *conn) insert() {
	log.Printf("inserting...")
	err := c.C.Insert(docs...)
	lerr := err.(*mgo.LastError)
	if lerr.Err != "" {
		log.Printf("insert err: %s (%#v)", lerr.Err, err)
		c.wr <- writeResult{n: uint(lerr.N), err: lerr.Err}
	}
	c.wr <- writeResult{n: uint(len(docs))}
}

func (c *conn) stepDown() {
	var res bson.D
	c.s.Run(bson.D{{"replSetStepDown", DEFAULT_STEPDOWN_TIME}, {"secondaryCatchUpPeriodSecs", DEFAULT_STEPDOWN_TIME / 2}}, res)
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
