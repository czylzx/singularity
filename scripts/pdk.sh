#!/usr/bin/env bash
#
# This script builds the application from source.
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

echo ${DIR}
# Change into that directory
cd $DIR

# Go File Name
GO_FILE="$1";
CONF_FILE="$2"

echo "Packaging ${GO_FILE} and ${CONF_FILE}..."

# Software version
VERSION="0.5.0"

# A pre-release marker for the version. If this is "" (empty string)
# then it means that it is a final release. Otherwise, this is a pre-release
# such as "dev", "beta", "rc1", etc.
VERSION_PRERELEASE="dev"

# Release version is combination of VERSION and VERSION_PRERELEASE
RELEASE_VERSION=$VERSION"-"$VERSION_PRERELEASE

# Get the mercurial commit
MERCURIAL_COMMIT=$(hg log | awk 'NR==1{print $2}' | sed 's/^[0-9]*://')

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

# Install dependencies
echo "--> Installing dependencies to speed up builds..."
go get \
  -ldflags "${CGO_LDFLAGS}" \
  ./...

# Build!
suffix=".go"
prefix="../"
OUT_FILE="${GO_FILE%$suffix}";
BUILD_FILE="$DIR""/""${GO_FILE#$prefix}"
echo "--> Building ${BUILD_FILE}"
go build -ldflags "${CGO_LDFLAGS} -X main.MercurialCommit=${MERCURIAL_COMMIT} -X main.ReleaseVersion=${RELEASE_VERSION} " \
  -v \
  -o ${OUT_FILE} ${EXTENSION} ${BUILD_FILE}

TAR_FILE="${OUT_FILE#$prefix}"".tar";
NCONF_FILE="${CONF_FILE#$prefix}"
echo "Creating ${TAR_FILE}"
tar -cf ${TAR_FILE} ${BUILD_FILE} ${NCONF_FILE}

echo "Build successful."
