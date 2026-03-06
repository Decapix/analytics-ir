package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"analytics-ir/event-collector/internal/buffer"
	"analytics-ir/event-collector/internal/clickhouse"
	"analytics-ir/event-collector/internal/model"
	"analytics-ir/event-collector/internal/session"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	buffer          *buffer.BatchBuffer
	ch              *clickhouse.Client
	sessionManager  *session.Manager
	flushSize       int
	flushInterval   time.Duration
	insertTimeout   time.Duration
	mu              sync.Mutex
	isFlushing      bool
}

func NewServer(chClient *clickhouse.Client, flushSize int, flushInterval, sessionWindow, insertTimeout time.Duration) *Server {
	buf := buffer.NewBatchBuffer(flushSize * 2)
	s := &Server{
		buffer:         buf,
		ch:             chClient,
		sessionManager: session.NewManager(sessionWindow),
		flushSize:      flushSize,
		flushInterval:  flushInterval,
		insertTimeout:  insertTimeout,
	}
	buf.StartPeriodicFlush(flushInterval, s.flushAsync)
	return s
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.POST("/events", s.ingestEvent)
	r.GET("/internal/percentages", s.getPercentages)
	r.GET("/internal/event-names", s.getEventNames)
	r.GET("/internal/last-events", s.getLastEvents)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
}

func (s *Server) ingestEvent(c *gin.Context) {
	var e model.AnalyticsEvent
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload", "details": err.Error()})
		return
	}

	now := time.Now().UTC()
	if e.Timestamp.IsZero() {
		e.Timestamp = now
	}
	if e.UserAgent == "" {
		e.UserAgent = c.GetHeader("User-Agent")
	}
	if e.IPHash == "" {
		e.IPHash = hashIP(c.ClientIP())
	}
	if e.Properties == nil {
		e.Properties = map[string]interface{}{}
	}
	if continued := s.sessionManager.Touch(e.SessionID, now); continued {
		e.Properties["session_continued"] = true
	}
	if _, ok := e.Properties["collector_event_id"]; !ok {
		e.Properties["collector_event_id"] = uuid.NewString()
	}

	size := s.buffer.Add(e)
	if size >= s.flushSize {
		s.flushAsync()
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

func (s *Server) getPercentages(c *gin.Context) {
	hours := 24
	if raw := c.Query("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			hours = parsed
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), s.insertTimeout)
	defer cancel()

	result, err := s.ch.QueryPercentages(ctx, hours)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "analytics unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"interval_hours": hours, "results": result})
}

func (s *Server) getEventNames(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), s.insertTimeout)
	defer cancel()

	names, err := s.ch.QueryEventNames(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "analytics unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(names), "event_names": names})
}

func (s *Server) getLastEvents(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	eventName := c.Query("event_name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), s.insertTimeout)
	defer cancel()

	events, err := s.ch.QueryLastEvents(ctx, limit, eventName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "analytics unavailable"})
		return
	}

	resp := gin.H{"limit": limit, "count": len(events), "events": events}
	if eventName != "" {
		resp["event_name"] = eventName
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) flushAsync() {
	s.mu.Lock()
	if s.isFlushing {
		s.mu.Unlock()
		return
	}
	s.isFlushing = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.isFlushing = false
			s.mu.Unlock()
		}()

		batch := s.buffer.Drain()
		if len(batch) == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), s.insertTimeout)
		defer cancel()

		err := clickhouse.RetryInsert(ctx, 3, 300*time.Millisecond, func(insertCtx context.Context) error {
			return s.ch.InsertBatch(insertCtx, batch)
		})
		if err != nil {
			log.Printf("failed to flush %d events: %v", len(batch), err)
		}
	}()
}

func hashIP(rawIP string) string {
	if rawIP == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(rawIP)
	if err == nil {
		rawIP = host
	}
	h := sha256.Sum256([]byte(rawIP))
	return hex.EncodeToString(h[:])
}
