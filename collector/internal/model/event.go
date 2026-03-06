package model

import "time"

// AnalyticsEvent is the normalized event received by the collector.
type AnalyticsEvent struct {
	Timestamp       time.Time              `json:"timestamp"`
	EventName       string                 `json:"event_name" binding:"required"`
	EntityType      string                 `json:"entity_type" binding:"required"`
	EntityID        string                 `json:"entity_id" binding:"required"`
	ActorUserID     string                 `json:"actor_user_id"`
	ActorEntityID   string                 `json:"actor_entity_id"`
	ActorEntityType string                 `json:"actor_entity_type"`
	SessionID       string                 `json:"session_id" binding:"required"`
	Source          string                 `json:"source" binding:"required"`
	Platform        string                 `json:"platform" binding:"required"`
	AppVersion      string                 `json:"app_version"`
	DeviceType      string                 `json:"device_type"`
	UserAgent       string                 `json:"user_agent"`
	IPHash          string                 `json:"ip_hash"`
	Properties      map[string]interface{} `json:"properties"`
}
