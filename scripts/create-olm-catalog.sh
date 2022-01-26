#!/usr/bin/env bash
set -e

CATALOG_DIR=infinispan-olm-catalog
DOCKERFILE=${CATALOG_DIR}.Dockerfile
CATALOG=${CATALOG_DIR}/catalog.json
BUNDLE=${CATALOG_DIR}/bundle.json

mkdir -p ${CATALOG_DIR}

${OPM} render --skip-tls-verify ${CATALOG_BASE_IMG} > ${CATALOG}
${OPM} render --skip-tls-verify ${BUNDLE_IMGS} > ${BUNDLE}
# cp /tmp/bundle.json ${BUNDLE}
# cp /tmp/catalog.json ${CATALOG}

default_channel=$(jq 'select((.name=="infinispan") and (.schema=="olm.package"))' ${CATALOG} | jq -r .defaultChannel)
channel_selector="(.package==\"infinispan\") and (.schema==\"olm.channel\") and (.name==\"${default_channel}\")"
old_channel=$(jq -r "select(${channel_selector})" ${CATALOG})
old_channel_latest_bundle=$(echo $old_channel | jq -r '.entries[-1].name')

bundle_name=$(jq -r .name ${BUNDLE})
new_channel=$(echo $old_channel | jq ".entries += [{\"name\":\"${bundle_name}\", \"replaces\":\"${old_channel_latest_bundle}\"}]")

new_catalog=$(jq -r "select(${channel_selector} | not)" ${CATALOG})
echo ${new_channel} ${new_catalog} > ${CATALOG_DIR}/new_catalog.json

# Remove the original catalog no longer required
rm -f ${CATALOG}

${OPM} validate ${CATALOG_DIR}
${OPM} generate dockerfile ${CATALOG_DIR}
${CONTAINER_TOOL} build -f ${DOCKERFILE} -t ${CATALOG_IMG} .

rm -rf ${DOCKERFILE}
