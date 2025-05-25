#!/usr/bin/env sh

# Script need to run from root folder!

export CUR="github.com/kobu/repo-int"
export NEW="github.com/kobu/dm-repo-integration"
go mod edit -module ${NEW}
find . -type f -name '*.go' -exec perl -pi -e 's/$ENV{CUR}/$ENV{NEW}/g' {} \;