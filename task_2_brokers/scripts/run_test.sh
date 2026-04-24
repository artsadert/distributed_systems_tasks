#!/bin/bash

BROKER=$1
URI=$2
QUEUE_PREFIX=$3   # базовое имя, к нему добавим _size_rate

if [ -z "$BROKER" ] || [ -z "$URI" ] || [ -z "$QUEUE_PREFIX" ]; then
    echo "Usage: $0 <rabbitmq|redis> <connection_uri> <queue_prefix>"
    exit 1
fi

SIZES=("128" "1024" "10240" "102400")
RATES=("1000" "5000" "10000")
DURATION_SEC=30
SLEEP_DELAY=2
DRAIN_SEC=5
PRODUCER_DURATION="${DURATION_SEC}s"
# Consumer runs longer: covers the sleep delay before producer starts + drain time after producer stops
CONSUMER_DURATION="$((DURATION_SEC + SLEEP_DELAY + DRAIN_SEC))s"

mkdir -p results

for SIZE in "${SIZES[@]}"; do
    for RATE in "${RATES[@]}"; do
        QUEUE="${QUEUE_PREFIX}_${SIZE}_${RATE}"
        METRICS_PREFIX="${BROKER}_size${SIZE}_rate${RATE}"
        echo "Test: Broker=$BROKER, Size=$SIZE, Rate=$RATE, Queue=$QUEUE"

        ./bin/consumer \
            --broker "$BROKER" \
            --uri "$URI" \
            --queue "$QUEUE" \
            --duration "$CONSUMER_DURATION" \
            --metrics-file "results/${METRICS_PREFIX}_consumer.json" \
            --log-level warn &
        CONSUMER_PID=$!

        sleep $SLEEP_DELAY

        ./bin/producer \
            --broker "$BROKER" \
            --uri "$URI" \
            --queue "$QUEUE" \
            --size "$SIZE" \
            --rate "$RATE" \
            --duration "$PRODUCER_DURATION" \
            --metrics-file "results/${METRICS_PREFIX}_producer.json" \
            --log-level info

        wait $CONSUMER_PID
    done
done