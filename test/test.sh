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
trap cleanup EXIT

../pastad &
sleep 1        # Don't do sleep you lazy :-)
echo "Testfile 123" > testfile
../pasta -r http://localhost:8199 < testfile > pasta.json
cat pasta.json
link=`jq -r .url pasta.json`
echo $link
curl -o testfile2 $link
diff testfile testfile2
echo "Testfile matches"
echo "Testfile 123456789" > testfile
../pasta -r http://localhost:8199 < testfile > pasta.json
link=`jq -r .url pasta.json`
curl -o testfile2 $link
diff testfile testfile2
echo "Testfile 2 matches"

echo "All good :-)"
