These files contain info about a MySQL memory leak:

* `massif.txt`: Valgrind Massif report of a MySQL instance when leak was reproduced.
* `my.cnf`: MySQL config for that ^ instance.
* `global_vars.txt`: `SHOW GLOBAL VARIABLES` from that ^ instance.
* `rss.log.csv`: RSS at 20s intervals during the memory leak on that ^ instance.
