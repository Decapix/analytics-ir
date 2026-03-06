package clickhouse

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"analytics-ir/event-collector/internal/model"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

type Client struct {
	conn ch.Conn
}

func NewClient(addr, user, password, database string) (*Client, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{Database: database, Username: user, Password: password},
		Settings: ch.Settings{
			"async_insert": 1,
		},
		DialTimeout:      3 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		Compression: &ch.Compression{
			Method: ch.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) InsertBatch(ctx context.Context, events []model.AnalyticsEvent) error {
	batch, err := c.conn.PrepareBatch(ctx, `INSERT INTO analytics.analytics_events (
		timestamp,event_name,entity_type,entity_id,
		actor_user_id,actor_entity_id,actor_entity_type,
		session_id,source,platform,app_version,device_type,
		user_agent,ip_hash,properties
	) VALUES`)
	if err != nil {
		return err
	}

	for _, e := range events {
		if e.Timestamp.IsZero() {
			e.Timestamp = time.Now().UTC()
		}
		propertiesJSON, _ := json.Marshal(e.Properties)
		if err = batch.Append(
			e.Timestamp,
			e.EventName,
			e.EntityType,
			e.EntityID,
			e.ActorUserID,
			e.ActorEntityID,
			e.ActorEntityType,
			e.SessionID,
			e.Source,
			e.Platform,
			e.AppVersion,
			e.DeviceType,
			e.UserAgent,
			e.IPHash,
			string(propertiesJSON),
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (c *Client) QueryPercentages(ctx context.Context, intervalHours int) ([]map[string]interface{}, error) {
	rows, err := c.conn.Query(ctx, `
		SELECT
			event_name,
			count() AS total,
			round(100.0 * count() / sum(count()) OVER (), 2) AS percentage
		FROM analytics.analytics_events
		WHERE timestamp >= now() - toIntervalHour(?)
		GROUP BY event_name
		ORDER BY total DESC
	`, intervalHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		var eventName string
		var total uint64
		var percentage float64
		if err = rows.Scan(&eventName, &total, &percentage); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"event_name": eventName,
			"total":      total,
			"percentage": percentage,
		})
	}
	return result, nil
}

func (c *Client) QueryEventNames(ctx context.Context) ([]string, error) {
	rows, err := c.conn.Query(ctx, `
		SELECT DISTINCT event_name
		FROM analytics.analytics_events
		ORDER BY event_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (c *Client) QueryLastEvents(ctx context.Context, limit int, eventName string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			timestamp, event_name, entity_type, entity_id,
			actor_user_id, actor_entity_id, actor_entity_type,
			session_id, source, platform, app_version, device_type,
			user_agent, ip_hash, properties
		FROM analytics.analytics_events
		ORDER BY timestamp DESC
		LIMIT ?`
	args := []interface{}{limit}

	if eventName != "" {
		query = `
		SELECT
			timestamp, event_name, entity_type, entity_id,
			actor_user_id, actor_entity_id, actor_entity_type,
			session_id, source, platform, app_version, device_type,
			user_agent, ip_hash, properties
		FROM analytics.analytics_events
		WHERE event_name = ?
		ORDER BY timestamp DESC
		LIMIT ?`
		args = []interface{}{eventName, limit}
	}

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		var (
			timestamp       time.Time
			eventNameVal    string
			entityType      string
			entityID        string
			actorUserID     string
			actorEntityID   string
			actorEntityType string
			sessionID       string
			source          string
			platform        string
			appVersion      string
			deviceType      string
			userAgent       string
			ipHash          string
			properties      string
		)
		if err = rows.Scan(
			&timestamp, &eventNameVal, &entityType, &entityID,
			&actorUserID, &actorEntityID, &actorEntityType,
			&sessionID, &source, &platform, &appVersion, &deviceType,
			&userAgent, &ipHash, &properties,
		); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"timestamp":         timestamp.UTC().Format(time.RFC3339),
			"event_name":        eventNameVal,
			"entity_type":       entityType,
			"entity_id":         entityID,
			"actor_user_id":     actorUserID,
			"actor_entity_id":   actorEntityID,
			"actor_entity_type": actorEntityType,
			"session_id":        sessionID,
			"source":            source,
			"platform":          platform,
			"app_version":       appVersion,
			"device_type":       deviceType,
			"user_agent":        userAgent,
			"ip_hash":           ipHash,
			"properties":        properties,
		})
	}
	return result, nil
}

func RetryInsert(ctx context.Context, retries int, wait time.Duration, insertFn func(context.Context) error) error {
	var err error
	for i := 0; i < retries; i++ {
		err = insertFn(ctx)
		if err == nil {
			return nil
		}
		log.Printf("clickhouse insert retry=%d err=%v", i+1, err)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}
