# server-29861

This is a debug program for https://jira.mongodb.org/browse/SERVER-29861

Easiest use case is on replica set with no authentication and 3 nodes.

If replica set name is "rs0", run like:

```
./server-29861 mongodb://localhost?replicaSet=rs0 1>data.csv 2>log
```

Log is printed to STDERR, csv results are printed to STDOUT.

The csv data has fields: `nReported,nActual,secondsBeforeStepDown,errorString,pass`

* nReported = the number of docs written ("n") as reported by the modified, vendored-in copy of [mgo](http://labix.org/mgo). You can see my mods at/btween @dn code comments. Basically, it just makes mgo actually report "n" from MongoDB (which is a separate issue for mgo).
* nActual = the number of docs removed as reported by MongoDB, after the repl set has recovered, by doing db.remove() on the (new) primary. This number is authoritative, how many docs are truly stored in the collection.
* secondsBeforeStepDown = random wait between starting insert and doing rs.stopDown on master, [500ms, 2).
* errorString = "int" = "operation was interrupted", "eof" = "EOF" (socket), "aok" = all docs inserted before secondsBeforeStepDown (rare given 250k docs are inserted)
* pass = "true" or "false" depending on if nReported = nActual

The line number of the csv data = the test number (important to match results with log).
