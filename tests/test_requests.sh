
# Test d'envoi d'événement
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
    "event_name": "page_view",
    "entity_type": "page",
    "entity_id": "home",
    "session_id": "session_123",
    "source": "web",
    "platform": "desktop",
    "properties": {"page": "/home", "user_id": "user123"}
  }'

echo ""

# Test de santé
curl http://localhost:8080/healthz
