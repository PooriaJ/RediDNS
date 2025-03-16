package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PooriaJ/RediDNS/config"
	"github.com/PooriaJ/RediDNS/db"
	"github.com/PooriaJ/RediDNS/models"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

// DNSServer represents the DNS server
type DNSServer struct {
	cfg           *config.Config
	redisClient   *db.RedisClient
	mariadbClient *db.MariaDBClient
	logger        *logrus.Logger
	server        *dns.Server
	handler       *DNSHandler
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewDNSServer creates a new DNS server
func NewDNSServer(cfg *config.Config, redisClient *db.RedisClient, mariadbClient *db.MariaDBClient, logger *logrus.Logger) (*DNSServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	handler := NewDNSHandler(redisClient, mariadbClient, logger)

	return &DNSServer{
		cfg:           cfg,
		redisClient:   redisClient,
		mariadbClient: mariadbClient,
		logger:        logger,
		handler:       handler,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Start starts the DNS server
func (s *DNSServer) Start() error {
	// Create DNS server
	addr := fmt.Sprintf("%s:%d", s.cfg.DNS.Address, s.cfg.DNS.Port)
	s.server = &dns.Server{
		Addr:    addr,
		Net:     s.cfg.DNS.Protocol,
		Handler: s.handler,
	}

	// Start listening for record updates from Redis
	go s.listenForRecordUpdates()

	// Start DNS server
	s.logger.Infof("Starting DNS server on %s (%s)", addr, s.cfg.DNS.Protocol)
	return s.server.ListenAndServe()
}

// Stop stops the DNS server
func (s *DNSServer) Stop() {
	s.cancel()
	if s.server != nil {
		s.logger.Info("Shutting down DNS server")
		s.server.Shutdown()
	}
}

// listenForRecordUpdates listens for record updates from Redis pub/sub
func (s *DNSServer) listenForRecordUpdates() {
	pubsub := s.redisClient.SubscribeToRecordUpdates(s.ctx)
	defer pubsub.Close()

	// Listen for messages
	ch := pubsub.Channel()
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-ch:
			s.logger.Debugf("Received record update: %s", msg.Payload)

			// Parse the record update
			var record models.Record
			if err := json.Unmarshal([]byte(msg.Payload), &record); err != nil {
				s.logger.Errorf("Failed to parse record update: %v", err)
				continue
			}

			// Invalidate cache for this record
			ctx := context.Background()

			// Invalidate single record cache
			singleCacheKey := fmt.Sprintf("dns:record:%s:%s:%s", record.Zone, record.Name, record.Type)
			if err := s.redisClient.Del(ctx, singleCacheKey); err != nil {
				s.logger.Warnf("Failed to invalidate single record cache: %v", err)
			}

			// Invalidate multiple records cache
			multiCacheKey := fmt.Sprintf("dns:records:%s:%s:%s", record.Zone, record.Name, record.Type)
			if err := s.redisClient.Del(ctx, multiCacheKey); err != nil {
				s.logger.Warnf("Failed to invalidate multiple records cache: %v", err)
			}
		}
	}
}

// ReloadZones reloads all zones from the database
func (s *DNSServer) ReloadZones() error {
	// Implementation would depend on how zones are stored and managed
	return nil
}

// GetStats returns statistics about the DNS server
func (s *DNSServer) GetStats() map[string]interface{} {
	// Implement statistics collection
	return map[string]interface{}{
		"uptime": time.Since(time.Now()), // This is just a placeholder
	}
}
