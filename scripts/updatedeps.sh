#!/usr/bin/env bash
#
# This Script Update the Vendor List as per the Latest dependency

# External Repo
EXTREPO="github.com\|bitbucket.com"

DEPS=$(go list -f '{{join .Deps "\n"}}' | grep $EXTREPO)
# Install dependencies (while using go vendoring no need to get Latest packages)
echo "--> Updating Vendor List..."
rm -rf vendor
rm -rf bin
for DEP in $DEPS
do
        echo "Getting: $DEP"
        go get \
                -ldflags "${CGO_LDFLAGS}" \
                $DEP
	path="$(printenv GOPATH)/src/$DEP"
	echo "$path"
	mkdir -p $( dirname ./vendor/$DEP )
	cp -r $path ./vendor/$DEP

done
echo "--> Successfully Updated all the Vendor Repo"
