package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/PooriaJ/RediDNS/db"
	"github.com/PooriaJ/RediDNS/models"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

// DNSHandler handles DNS queries
type DNSHandler struct {
	redisClient   *db.RedisClient
	mariadbClient *db.MariaDBClient
	logger        *logrus.Logger
	stats         *DNSStats
}

// DNSStats holds statistics about DNS queries
type DNSStats struct {
	Queries       int64
	CacheHits     int64
	CacheMisses   int64
	NXDomain      int64
	ServerFailure int64
}

// NewDNSHandler creates a new DNS handler
func NewDNSHandler(redisClient *db.RedisClient, mariadbClient *db.MariaDBClient, logger *logrus.Logger) *DNSHandler {
	return &DNSHandler{
		redisClient:   redisClient,
		mariadbClient: mariadbClient,
		logger:        logger,
		stats:         &DNSStats{},
	}
}

// ServeDNS implements the dns.Handler interface
func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	h.stats.Queries++

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	// Process each question
	for _, q := range r.Question {
		h.logger.Debugf("Received query: %s %s %s", q.Name, dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass])

		// Handle the query
		if err := h.handleQuery(m, &q); err != nil {
			h.logger.Errorf("Error handling query: %v", err)
			m.Rcode = dns.RcodeServerFailure
			h.stats.ServerFailure++
		}
	}

	// If no answers were found, set NXDOMAIN
	if len(m.Answer) == 0 {
		m.Rcode = dns.RcodeNameError
		h.stats.NXDomain++
	}

	// Write response
	if err := w.WriteMsg(m); err != nil {
		h.logger.Errorf("Error writing DNS response: %v", err)
	}
}

// handleQuery processes a single DNS query
func (h *DNSHandler) handleQuery(m *dns.Msg, q *dns.Question) error {
	// Normalize the query name (remove trailing dot)
	name := strings.TrimSuffix(q.Name, ".")

	// Find the zone for this query
	zone, err := h.findZone(name)
	if err != nil {
		return err
	}

	if zone == "" {
		// No zone found for this query
		return nil
	}

	// Try to get records from cache first
	ctx := context.Background()
	recordType := models.RecordType(dns.TypeToString[q.Qtype])

	// Try to get multiple records from cache
	records, err := h.redisClient.GetRecordsByNameAndType(ctx, zone, name, recordType)
	if err == nil && len(records) > 0 {
		// Cache hit for multiple records
		h.stats.CacheHits++
		for _, record := range records {
			if err := h.addAnswerFromRecord(m, &record, q); err != nil {
				h.logger.Warnf("Failed to add answer from record: %v", err)
			}
		}
		return nil
	}

	// Try single record cache for backward compatibility
	record, err := h.redisClient.GetRecord(ctx, zone, name, recordType)
	if err == nil && record != nil {
		// Cache hit for single record
		h.stats.CacheHits++
		return h.addAnswerFromRecord(m, record, q)
	}

	// Cache miss, try to get from database
	h.stats.CacheMisses++

	// Get multiple records from database
	records, err = h.mariadbClient.GetRecordsByNameAndType(zone, name, recordType)
	if err != nil {
		return err
	}

	if len(records) > 0 {
		// Store multiple records in cache for future queries
		ttl := time.Duration(records[0].TTL) * time.Second
		if err := h.redisClient.SetRecords(ctx, records, ttl); err != nil {
			h.logger.Warnf("Failed to cache records: %v", err)
		}

		// Add all records to the answer
		for _, record := range records {
			if err := h.addAnswerFromRecord(m, &record, q); err != nil {
				h.logger.Warnf("Failed to add answer from record: %v", err)
			}
		}
		return nil
	}

	// Try to get a single record for backward compatibility
	record, err = h.mariadbClient.GetRecord(zone, name, recordType)
	if err != nil {
		return err
	}

	if record != nil {
		// Store in cache for future queries
		ttl := time.Duration(record.TTL) * time.Second
		if err := h.redisClient.SetRecord(ctx, record, ttl); err != nil {
			h.logger.Warnf("Failed to cache record: %v", err)
		}

		return h.addAnswerFromRecord(m, record, q)
	}

	// No record found
	return nil
}

// findZone finds the appropriate zone for a given name
func (h *DNSHandler) findZone(name string) (string, error) {
	// Split the name into parts
	parts := strings.Split(name, ".")

	// Try to find the zone by checking each level
	for i := 0; i < len(parts); i++ {
		potentialZone := strings.Join(parts[i:], ".")
		zone, err := h.mariadbClient.GetZone(potentialZone)
		if err != nil {
			return "", err
		}

		if zone != nil {
			return zone.Name, nil
		}
	}

	return "", nil
}

// addAnswerFromRecord adds a DNS answer from a record
func (h *DNSHandler) addAnswerFromRecord(m *dns.Msg, record *models.Record, q *dns.Question) error {
	switch record.Type {
	case models.TypeA:
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			A: net.ParseIP(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeAAAA:
		rr := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			AAAA: net.ParseIP(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeCNAME:
		rr := &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Target: dns.Fqdn(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeMX:
		rr := &dns.MX{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeMX,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Preference: uint16(record.Priority),
			Mx:         dns.Fqdn(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeNS:
		rr := &dns.NS{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Ns: dns.Fqdn(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypePTR:
		rr := &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Ptr: dns.Fqdn(record.Content),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeTXT:
		rr := &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Txt: []string{record.Content},
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeSOA:
		// Parse SOA record content
		var soaData models.SOARecord
		if err := json.Unmarshal([]byte(record.Content), &soaData); err != nil {
			return fmt.Errorf("failed to parse SOA record: %w", err)
		}

		rr := &dns.SOA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Ns:      dns.Fqdn(soaData.Mname),
			Mbox:    dns.Fqdn(soaData.Rname),
			Serial:  soaData.Serial,
			Refresh: soaData.Refresh,
			Retry:   soaData.Retry,
			Expire:  soaData.Expire,
			Minttl:  soaData.Minimum,
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeSRV:
		// Parse SRV record content
		var srv models.SRVRecord
		if err := json.Unmarshal([]byte(record.Content), &srv); err != nil {
			return fmt.Errorf("failed to parse SRV record: %w", err)
		}

		rr := &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Priority: srv.Priority,
			Weight:   srv.Weight,
			Port:     srv.Port,
			Target:   dns.Fqdn(srv.Target),
		}
		m.Answer = append(m.Answer, rr)

	case models.TypeCAA:
		// Parse CAA record content
		var caa models.CAARecord
		if err := json.Unmarshal([]byte(record.Content), &caa); err != nil {
			return fmt.Errorf("failed to parse CAA record: %w", err)
		}

		rr := &dns.CAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeCAA,
				Class:  dns.ClassINET,
				Ttl:    uint32(record.TTL),
			},
			Flag:  caa.Flag,
			Tag:   caa.Tag,
			Value: caa.Value,
		}
		m.Answer = append(m.Answer, rr)

	default:
		return fmt.Errorf("unsupported record type: %s", record.Type)
	}

	return nil
}

// GetStats returns the current DNS statistics
func (h *DNSHandler) GetStats() *DNSStats {
	return h.stats
}
