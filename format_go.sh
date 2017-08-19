#!/bin/bash

unset dirs files
dirs=$(go list -f {{.Dir}} ./... | grep -v /vendor/)
    for d in $dirs
do
    for f in $d/*.go
    do
        if ! [[ $f =~ ^.+\/helpers\/assets\.go$ ]]; then
            files="${files} $f"
        fi
    done
done
diff <(gofmt -d $files) <(echo -n)

