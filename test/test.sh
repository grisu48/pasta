#!/bin/bash
#
# Summary: Function test for pasta
#

PASTAS=~/.pastas.dat               # pasta client dat file
PASTAS_TEMP=""                     # temp file, if present

function cleanup() {
	set +e
	# restore old pasta client file
	if [[ $PASTAS_TEMP != "" ]]; then
		mv "$PASTAS_TEMP" "$PASTAS"
	fi
	rm -f testfile
	rm -f testfile2
	rm -f rm
	kill %1
	rm -rf pasta_test
	rm -f pasta.json
}

set -e
#set -x
trap cleanup EXIT

## Preparation: Safe old pastas.dat, if existing
if [[ -s $PASTAS ]]; then
	PASTAS_TEMP=`mktemp`
	mv "$PASTAS" "$PASTAS_TEMP"
fi

## Setup pasta server
../pastad -c pastad.toml -m ../mime.types -B http://127.0.0.1:8200 -b 127.0.0.1:8200 &
sleep 1        # Don't do sleep here you lazy ... :-)

## Push a testfile
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

## Test spam protection
echo "Testing spam protection ... "
../pasta -r http://127.0.0.1:8200 testfile >/dev/null
! timeout 1 ../pasta -r http://127.0.0.1:8200 testfile >/dev/null
sleep 2
../pasta -r http://127.0.0.1:8200 testfile >/dev/null

## TODO: Test expire pasta cleanup

## Test special commands
function test_special_command() {
	command="$1"
	echo "test" > $command
	# Ambiguous, if the shortcut command and a similar file exists. This must fail
	! ../pasta -r http://127.0.0.1:8200 "$command"
	# However it must pass, if the file is explicitly stated
	../pasta -r http://127.0.0.1:8200 -f "$command"
	# And it must succeed, if there is no such file and thus is it clear what should happen
	if [[ "$command" != "rm" ]]; then rm "$command"; fi
	../pasta -r http://127.0.0.1:8200 "$command"
}
test_special_command "ls"
test_special_command "rm"
test_special_command "gc"

echo "All good :-)"
