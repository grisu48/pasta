#!/bin/bash
# Summary: Function test for pasta & pastad

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
	rm -f test_config.toml
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
sleep 2

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
# Test also sending via curl
url=`curl -X POST http://127.0.0.1:8200 --data-binary @testfile | grep -Eo 'http://.*'`
echo "curl stored as $url"
curl -o testfile3 "$url"
diff testfile testfile3
echo "Testfile 3 matches"
# Test the POST form
echo -n "testpasta" > testfile4
url=`curl -X POST "http://127.0.0.1:8200?input=form&content=testpasta" | grep -Eo 'http://.*'`
curl -o testfile5 "$url"
diff testfile4 testfile5
# Test different format in link
curl -X POST http://127.0.0.1:8200?ret=json --data-binary @testfile

## Second pasta server with environment variables
echo "Testing environment variables ... "
PASTA_BASEURL=pastas PASTA_BINDADDR=127.0.0.1:8201 PASTA_CHARACTERS=12 ../pastad -m ../mime.types &
SECONDPID=$!
sleep 2        # TODO: Don't do sleep here you lazy ... :-)
link2=`../pasta -r http://127.0.0.1:8201 < testfile`
curl -o testfile_second $link
diff testfile testfile_second
kill $SECONDPID

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

## Test creation of default config
rm -f test_config.toml
../pastad -c test_config.toml -B http://127.0.0.1:8201 -b 127.0.0.1:8201 &
sleep 2 # TODO: Don't sleep here either but create a valid monitor
kill %2
stat test_config.toml
# Ensure the test config contains the expected entries
grep 'BaseURL[[:space:]]=' test_config.toml
grep 'BindAddress[[:space:]]*=' test_config.toml
grep 'PastaDir[[:space:]]*=' test_config.toml
grep 'MaxPastaSize[[:space:]]*=' test_config.toml
grep 'PastaCharacters[[:space:]]*=' test_config.toml
grep 'Expire[[:space:]]*=' test_config.toml
grep 'Cleanup[[:space:]]*=' test_config.toml
grep 'RequestDelay[[:space:]]*=' test_config.toml
echo "test_config.toml has been successfully created"


echo "All good :-)"
