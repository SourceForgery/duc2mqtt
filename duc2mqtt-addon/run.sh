#!/bin/sh -eu

disallowed_prefixes_yaml=$(echo "${DUC_DISALLOWED_PREFIXES}" | sed 's/,/\n    - /g')

cat << EOF > /etc/duc2mqtt/config.yaml
mqtt:
  url: ${MQTT_URL}
  uniqueId: ${MQTT_UNIQUE_ID}
  topicPrefix: ${MQTT_TOPIC_PREFIX}
duc:
  url: ${DUC_URL}
  disallowedPrefixes:
    - ${disallowed_prefixes_yaml}
intervalSeconds: ${INTERVAL_SECONDS}
EOF

exec / -config /etc/duc2mqtt/config.yaml
