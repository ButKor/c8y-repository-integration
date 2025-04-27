#!/usr/bin/env sh

# Script need to run from root folder!

export CUR="github.com/kobu/c8y-repo-int" # example: github.com/user/old-lame-name
export NEW="github.com/kobu/repo-int" # example: github.com/user/new-super-cool-name
go mod edit -module ${NEW}
find . -type f -name '*.go' -exec perl -pi -e 's/$ENV{CUR}/$ENV{NEW}/g' {} \;