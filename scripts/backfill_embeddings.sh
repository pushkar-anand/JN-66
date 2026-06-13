#!/bin/bash
# Backfills embeddings for agent_memories rows that have NULL embedding.
# Uses the configured Ollama endpoint and nomic-embed-text model.

set -euo pipefail

DB_CONTAINER="${DB_CONTAINER:-jn-66-postgres-1}"
OLLAMA_URL="${OLLAMA_URL:-https://ollama.lab.pushkar.dev}"
MODEL="${MODEL:-nomic-embed-text}"

echo "Backfilling embeddings (container=$DB_CONTAINER, model=$MODEL)..."

docker exec "$DB_CONTAINER" psql -U finagent -d finagent -t -A -F$'\t' \
  -c "SELECT id, content FROM agent_memories WHERE embedding IS NULL AND is_active = TRUE" | \
while IFS=$'\t' read -r id content; do
    printf "  %s ... " "$id"
    vec=$(curl -sf "$OLLAMA_URL/api/embeddings" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$MODEL\",\"prompt\":$(echo "$content" | jq -R .)}" | \
        jq -r '[.embedding[] | tostring] | "[" + join(",") + "]"')

    if [ -z "$vec" ]; then
        echo "FAILED"
        exit 1
    fi

    docker exec "$DB_CONTAINER" psql -U finagent -d finagent -q \
        -c "UPDATE agent_memories SET embedding = '$vec'::vector WHERE id = '$id'"
    echo "OK"
done

echo "Done."
