# Frontend and Mobile tracking contract

Each event sent to FastAPI must include:

- `event_name`
- `entity_type`
- `entity_id`
- `actor_user_id`
- `actor_entity_id`
- `actor_entity_type`
- `session_id`
- `source`
- `platform`
- `app_version`
- `device_type`
- `properties` (optional JSON)

## Session ID generation

Generate a UUID on first app activity, keep it in memory + persistent storage for active app lifecycle, and rotate it after inactivity timeout.

## Event naming conventions

- Posts: `view_post`, `click_post`, `save_post`
- User profile: `view_user_profile`
- Brand/stylist/press office profile: `view_profile`, `click_profile_from_search`
- Collections: `view_collection`, `click_collection`, `save_collection`
- Looks: `view_look`, `click_look`, `save_look`
- Articles: `view_article`, `click_article`, `save_article`, `add_article_to_project`, `add_article_to_project_request`, `add_article_to_project_request_credit`
- Search analytics: `search_filters_used` (+ filters in `properties`)
- Brand search bar: `brand_search` (+ query in `properties`)
- Session: `session_start`, `session_end`

## Semantics

- `view_*` events are sent when card/item is visible on screen.
- `click_*` events represent navigation to a detail page.
- Never send raw IP to APIs; backend computes `ip_hash`.
