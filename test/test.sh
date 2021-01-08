#!/bin/bash
#
# Summary: Function test for pasta
#

function cleanup() {
	rm -f testfile
	rm -f testfile2
	kill %1
	rm -rf pasta_test
	rm -f pasta.json
}

set -e
#set -x
trap cleanup EXIT

../pastad -m ../mime.types -B http://127.0.0.1:8200 -b 127.0.0.1:8200 &
sleep 1        # Don't do sleep you lazy :-)
echo "Testfile 123" > testfile
link=`../pasta -r http://127.0.0.1:8200 < testfile`
curl -o testfile2 $link
diff testfile testfile2
echo "Testfile matches"
echo "Testfile 123456789" > testfile
link=`../pasta -r http://127.0.0.1:8200 < testfile`
curl -o testfile2 $link
diff testfile testfile2
echo "Testfile 2 matches"

echo "All good :-)"
