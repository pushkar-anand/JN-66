#!/bin/bash
# Backfills embeddings for agent_memories rows that have NULL embedding.
# Uses the configured Ollama endpoint and nomic-embed-text model.

set -euo pipefail

DB_CONTAINER="${DB_CONTAINER:-jn-66-postgres-1}"
OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
MODEL="${MODEL:-nomic-embed-text}"

echo "Backfilling embeddings (container=$DB_CONTAINER, model=$MODEL)..."

docker exec "$DB_CONTAINER" psql -U finagent -d finagent -t -A -F$'\t' \
  -c "SELECT id, content FROM agent_memories WHERE embedding IS NULL AND is_active = TRUE" | \
while IFS=$'\t' read -r id content; do
    printf "  %s ... " "$id"
    vec=$(curl -sf "$OLLAMA_URL/v1/embeddings" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ollama" \
        -d "{\"model\":\"$MODEL\",\"input\":$(echo "$content" | jq -R .)}" | \
        jq -r '[.data[0].embedding[] | tostring] | "[" + join(",") + "]"')

    if [ -z "$vec" ]; then
        echo "FAILED"
        exit 1
    fi

    docker exec "$DB_CONTAINER" psql -U finagent -d finagent -q \
        -c "UPDATE agent_memories SET embedding = '$vec'::vector WHERE id = '$id'"
    echo "OK"
done

echo "Done."
