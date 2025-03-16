package models

import (
	"time"
)

// RecordType represents the type of DNS record
type RecordType string

// DNS record types
const (
	TypeA     RecordType = "A"     // IPv4 address
	TypeAAAA  RecordType = "AAAA"  // IPv6 address
	TypeCNAME RecordType = "CNAME" // Canonical name
	TypeMX    RecordType = "MX"    // Mail exchange
	TypeNS    RecordType = "NS"    // Name server
	TypePTR   RecordType = "PTR"   // Pointer
	TypeSOA   RecordType = "SOA"   // Start of authority
	TypeSRV   RecordType = "SRV"   // Service
	TypeTXT   RecordType = "TXT"   // Text
	TypeCAA   RecordType = "CAA"   // Certification Authority Authorization
)

// Record represents a DNS record
type Record struct {
	ID        int64      `json:"id" db:"id"`
	Zone      string     `json:"zone" db:"zone"`
	Name      string     `json:"name" db:"name"`
	Type      RecordType `json:"type" db:"type"`
	Content   string     `json:"content" db:"content"`
	TTL       int        `json:"ttl" db:"ttl"`
	Priority  int        `json:"priority" db:"priority"` // Used for MX and SRV records
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// SOARecord represents a Start of Authority record
type SOARecord struct {
	Mname   string `json:"mname"`   // Primary master name server
	Rname   string `json:"rname"`   // Email address of the administrator
	Serial  uint32 `json:"serial"`  // Serial number
	Refresh uint32 `json:"refresh"` // Refresh interval
	Retry   uint32 `json:"retry"`   // Retry interval
	Expire  uint32 `json:"expire"`  // Expiration limit
	Minimum uint32 `json:"minimum"` // Minimum TTL
}

// MXRecord represents a Mail Exchange record
type MXRecord struct {
	Record
	Preference uint16 `json:"preference"` // Preference value
	Exchange   string `json:"exchange"`   // Mail server hostname
}

// SRVRecord represents a Service record
type SRVRecord struct {
	Record
	Priority uint16 `json:"priority"` // Priority value
	Weight   uint16 `json:"weight"`   // Weight value
	Port     uint16 `json:"port"`     // Port number
	Target   string `json:"target"`   // Target hostname
}

// CAARecord represents a Certification Authority Authorization record
type CAARecord struct {
	Record
	Flag  uint8  `json:"flag"`  // Flag
	Tag   string `json:"tag"`   // Tag
	Value string `json:"value"` // Value
}

// RecordSet represents a collection of DNS records
type RecordSet struct {
	Records []Record `json:"records"`
}

// Zone represents a DNS zone
type Zone struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
