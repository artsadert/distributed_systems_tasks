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
DURATION="30s"

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
            --duration "$DURATION" \
            --metrics-file "results/${METRICS_PREFIX}_consumer.json" \
            --log-level warn &
        CONSUMER_PID=$!
        
        sleep 2
        
        ./bin/producer \
            --broker "$BROKER" \
            --uri "$URI" \
            --queue "$QUEUE" \
            --size "$SIZE" \
            --rate "$RATE" \
            --duration "$DURATION" \
            --metrics-file "results/${METRICS_PREFIX}_producer.json" \
            --log-level info
        
        wait $CONSUMER_PID
    done
done