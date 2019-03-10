#!/bin/bash

# supermq standalone
docker run -p 1883:1883 -p 8083:8083 -p 8084:8084 -p 11883:11883 \
    -e MQTT_SERVER=tcp://0.0.0.0:1883 -e WS_MQTT_SERVER=0.0.0.0:8083 -e WSS_MQTT_SERVER=0.0.0.0:8084 \
    -e WEB_SERVER=0.0.0.0:11883 -e CERT_PATH=/go/src/github.com/578157900/supermq/certs/michael.crt \
    -e KEY_PATH=/go/src/github.com/578157900/supermq/certs/michael.key -e LOG_LEVEL=info -e LOG_DIR_LINUX=/home/supermq/ \
    -e LOG_PREFIX=supermq -e LOG_TO_FILE=false -e ROUTE_MODEL=false lumorechn/supermq:0.0.1