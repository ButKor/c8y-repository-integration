#!/usr/bin/env sh

# Script need to run from root folder!

export CUR="github.com/kobu/repo-int"
export NEW="github.com/kobu/c8y-devmgmt-repo-intgr"
go mod edit -module ${NEW}
find . -type f -name '*.go' -exec perl -pi -e 's/$ENV{CUR}/$ENV{NEW}/g' {} \;