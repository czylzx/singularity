#!/usr/bin/env bash
#
# This script builds the application from source.
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR

# GO Vendoring
export GO15VENDOREXPERIMENT=1

# Software version
VERSION="0.5.0"

# External Repo
EXTREPO="github.com\|bitbucket.com"

# A pre-release marker for the version. If this is "" (empty string)
# then it means that it is a final release. Otherwise, this is a pre-release
# such as "dev", "beta", "rc1", etc.
VERSION_PRERELEASE="dev"

# Release version is combination of VERSION and VERSION_PRERELEASE
RELEASE_VERSION=$VERSION"-"$VERSION_PRERELEASE

# Get the git commit
GIT_COMMIT=$(git rev-parse HEAD)

# If we're building on Windows, specify an extension
EXTENSION=""
if [ "$(go env GOOS)" = "windows" ]; then
    EXTENSION=".exe"
fi

GOPATHSINGLE=${GOPATH%%:*}
if [ "$(go env GOOS)" = "windows" ]; then
    GOPATHSINGLE=${GOPATH%%;*}
fi

if [ "$(go env GOOS)" = "freebsd" ]; then
	export CC="clang"
fi

# On OSX, we need to use an older target to ensure binaries are
# compatible with older linkers
if [ "$(go env GOOS)" = "darwin" ]; then
    export MACOSX_DEPLOYMENT_TARGET=10.6
fi

# Build!
echo "--> Building..."
go build \
  -ldflags "${CGO_LDFLAGS} -X main.GitCommit=${GIT_COMMIT} -X main.ReleaseVersion=${RELEASE_VERSION} " \
  -v \
  -o bin/singularity${EXTENSION}

cp bin/singularity${EXTENSION} ${GOPATHSINGLE}/bin

cp -r conf ./bin

mkdir plugin

mv plugin bin/

echo "Build successful."
