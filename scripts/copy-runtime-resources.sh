#!/bin/sh

set -eu

destination=${1:?destination directory is required}
source_directory=${2:-resources}
admin_policy=${3:-optional-admin}
client_policy=${4:-optional-client}

for required in i18n public templates; do
    if [ ! -d "${source_directory}/${required}" ]; then
        echo "missing runtime resource directory: ${source_directory}/${required}" >&2
        exit 1
    fi
done
if [ ! -f "${source_directory}/version" ]; then
    echo "missing runtime resource file: ${source_directory}/version" >&2
    exit 1
fi
if [ "${admin_policy}" = "require-admin" ] && [ ! -d "${source_directory}/admin" ]; then
    echo "reviewed admin build is required" >&2
    exit 1
fi
if [ "${client_policy}" = "require-client" ]; then
    if [ ! -s "${source_directory}/client/index.html" ] || \
       [ ! -s "${source_directory}/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt" ]; then
        echo "reviewed web client build and third-party licence text are required" >&2
        exit 1
    fi
fi

mkdir -p "${destination}"
cp -a "${source_directory}/i18n" "${destination}/"
cp -a "${source_directory}/public" "${destination}/"
cp -a "${source_directory}/templates" "${destination}/"
cp -a "${source_directory}/version" "${destination}/version"
if [ -d "${source_directory}/admin" ]; then
    cp -a "${source_directory}/admin" "${destination}/admin"
fi
if [ -d "${source_directory}/client" ]; then
    cp -a "${source_directory}/client" "${destination}/client"
fi

if [ -e "${destination}/web" ] || [ -e "${destination}/web2" ]; then
    echo "forbidden historical browser-client assets entered runtime resources" >&2
    exit 1
fi
