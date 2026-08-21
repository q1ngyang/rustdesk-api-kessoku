#!/bin/sh

set -eu

destination=${1:?destination directory is required}
source_directory=${2:-resources}
admin_policy=${3:-optional-admin}

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

mkdir -p "${destination}"
cp -a "${source_directory}/i18n" "${destination}/"
cp -a "${source_directory}/public" "${destination}/"
cp -a "${source_directory}/templates" "${destination}/"
cp -a "${source_directory}/version" "${destination}/version"
if [ -d "${source_directory}/admin" ]; then
    cp -a "${source_directory}/admin" "${destination}/admin"
fi

if [ -e "${destination}/web" ] || [ -e "${destination}/web2" ]; then
    echo "bundled browser-client assets entered runtime resources" >&2
    exit 1
fi
