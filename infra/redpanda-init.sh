#!/bin/bash
set -e

echo "Waiting for Redpanda broker..."
until rpk -X admin.hosts=redpanda:9644 cluster health 2>&1 | grep -qE "Healthy:\s+true"; do
  echo "  Broker not ready — retrying in 2s..."
  sleep 2
done
echo "Broker healthy."

# IMPORTANT: Docker Compose interpolation does NOT support arithmetic.
# Partition count MUST be computed here in the shell script.
PARTITIONS=$(( PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER ))
echo "Creating topic driver.location with ${PARTITIONS} partitions..."

rpk -X brokers=redpanda:9092 topic create driver.location \
  --partitions "${PARTITIONS}" \
  --replicas 1 \
  2>&1 | grep -vE "TOPIC_ALREADY_EXISTS|already exists" || true

echo "Init complete."
