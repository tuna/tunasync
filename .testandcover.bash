#!/bin/bash


function die() {
  echo $*
  exit 1
}

export GOPATH=`pwd`:$GOPATH

make

# Initialize profile.cov
echo "mode: count" > profile.cov

# Initialize error tracking
ERROR=""

# Test each package and append coverage profile info to profile.cov
for pkg in `cat .testpackages.txt`
do
    #$HOME/gopath/bin/
    go test -v -covermode=count -coverprofile=profile_tmp.cov $pkg || ERROR="Error testing $pkg"

    [ -f profile_tmp.cov ] && {
		tail -n +2 profile_tmp.cov >> profile.cov || die "Unable to append coverage for $pkg"
	}
done

if [ ! -z "$ERROR" ]
then
    die "Encountered error, last error was: $ERROR"
fi
