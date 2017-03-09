#! /bin/bash

sed -i "s/@KAFKA_BROKERS@/${KAFKA_BROKERS}/g" ./template/conf.gotmpl

exec ./wise-logger -stderrthreshold=INFO