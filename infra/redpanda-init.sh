#!/bin/bash
set -e

echo "Waiting for Redpanda broker..."
until rpk cluster health --brokers redpanda:9092 2>&1 | grep -q "Healthy: true"; do
  echo "  Broker not ready — retrying in 2s..."
  sleep 2
done
echo "Broker healthy."

# IMPORTANT: Docker Compose interpolation does NOT support arithmetic.
# Partition count MUST be computed here in the shell script.
PARTITIONS=$(( PUSH_SERVER_REPLICAS * PARTITION_MULTIPLIER ))
echo "Creating topic driver.location with ${PARTITIONS} partitions..."

rpk topic create driver.location \
  --partitions "${PARTITIONS}" \
  --replicas 1 \
  --brokers redpanda:9092 \
  --if-not-exists

echo "Init complete."
