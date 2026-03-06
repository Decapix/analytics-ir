curl http://localhost:8080/internal/event-names
{"count":10,"event_names":["add_to_cart","button_click","checkout","form_submit","login","logout","page_view","purchase","search","signup"]}



GET /internal/last-events?limit=5
GET /internal/last-events?limit=3&event_name=page_view
