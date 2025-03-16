package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/PooriaJ/RediDNS/config"
	"github.com/PooriaJ/RediDNS/models"
	_ "github.com/go-sql-driver/mysql"
)

// MariaDBClient wraps the MariaDB client with DNS server specific operations
type MariaDBClient struct {
	db *sql.DB
}

// NewMariaDBClient creates a new MariaDB client
func NewMariaDBClient(cfg *config.Config) (*MariaDBClient, error) {
	// Create DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.MariaDB.User,
		cfg.MariaDB.Password,
		cfg.MariaDB.Host,
		cfg.MariaDB.Port,
		cfg.MariaDB.DBName,
	)

	// Open database connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MariaDB: %w", err)
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping MariaDB: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &MariaDBClient{db: db}, nil
}

// Close closes the MariaDB connection
func (m *MariaDBClient) Close() error {
	return m.db.Close()
}

// InitSchema initializes the database schema if it doesn't exist
func (m *MariaDBClient) InitSchema() error {
	// Create zones table
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS zones (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return fmt.Errorf("failed to create zones table: %w", err)
	}

	// Create records table
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id INT AUTO_INCREMENT PRIMARY KEY,
			zone VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(10) NOT NULL,
			content TEXT NOT NULL,
			ttl INT NOT NULL DEFAULT 3600,
			priority INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX (zone, name, type),
			FOREIGN KEY (zone) REFERENCES zones(name) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`)
	if err != nil {
		return fmt.Errorf("failed to create records table: %w", err)
	}

	return nil
}

// GetZone retrieves a zone by name
func (m *MariaDBClient) GetZone(name string) (*models.Zone, error) {
	var zone models.Zone
	err := m.db.QueryRow("SELECT id, name, created_at, updated_at FROM zones WHERE name = ?", name).Scan(
		&zone.ID, &zone.Name, &zone.CreatedAt, &zone.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Zone not found
		}
		return nil, err
	}

	return &zone, nil
}

// CreateZone creates a new zone
func (m *MariaDBClient) CreateZone(name string) (*models.Zone, error) {
	result, err := m.db.Exec("INSERT INTO zones (name) VALUES (?)", name)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Zone{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// DeleteZone deletes a zone and all its records
func (m *MariaDBClient) DeleteZone(name string) error {
	_, err := m.db.Exec("DELETE FROM zones WHERE name = ?", name)
	return err
}

// GetRecord retrieves a record by zone, name, and type
func (m *MariaDBClient) GetRecord(zone, name string, recordType models.RecordType) (*models.Record, error) {
	var record models.Record
	err := m.db.QueryRow(
		"SELECT id, zone, name, type, content, ttl, priority, created_at, updated_at FROM records WHERE zone = ? AND name = ? AND type = ?",
		zone, name, recordType,
	).Scan(
		&record.ID, &record.Zone, &record.Name, &record.Type, &record.Content,
		&record.TTL, &record.Priority, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Record not found
		}
		return nil, err
	}

	return &record, nil
}

// GetRecordsByNameAndType retrieves all records matching a zone, name, and type
func (m *MariaDBClient) GetRecordsByNameAndType(zone, name string, recordType models.RecordType) ([]models.Record, error) {
	rows, err := m.db.Query(
		"SELECT id, zone, name, type, content, ttl, priority, created_at, updated_at FROM records WHERE zone = ? AND name = ? AND type = ?",
		zone, name, recordType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.Record
	for rows.Next() {
		var record models.Record
		err := rows.Scan(
			&record.ID, &record.Zone, &record.Name, &record.Type, &record.Content,
			&record.TTL, &record.Priority, &record.CreatedAt, &record.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// GetRecordsByZone retrieves all records for a specific zone
func (m *MariaDBClient) GetRecordsByZone(zone string) ([]models.Record, error) {
	rows, err := m.db.Query(
		"SELECT id, zone, name, type, content, ttl, priority, created_at, updated_at FROM records WHERE zone = ?",
		zone,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.Record
	for rows.Next() {
		var record models.Record
		err := rows.Scan(
			&record.ID, &record.Zone, &record.Name, &record.Type, &record.Content,
			&record.TTL, &record.Priority, &record.CreatedAt, &record.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// CreateRecord creates a new DNS record
func (m *MariaDBClient) CreateRecord(record *models.Record) error {
	result, err := m.db.Exec(
		"INSERT INTO records (zone, name, type, content, ttl, priority) VALUES (?, ?, ?, ?, ?, ?)",
		record.Zone, record.Name, record.Type, record.Content, record.TTL, record.Priority,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	record.ID = id
	return nil
}

// UpdateRecord updates an existing DNS record
func (m *MariaDBClient) UpdateRecord(record *models.Record) error {
	_, err := m.db.Exec(
		"UPDATE records SET content = ?, ttl = ?, priority = ? WHERE id = ?",
		record.Content, record.TTL, record.Priority, record.ID,
	)
	return err
}

// GetRecordByID retrieves a record by its ID
func (m *MariaDBClient) GetRecordByID(id int64) (*models.Record, error) {
	var record models.Record
	err := m.db.QueryRow(
		"SELECT id, zone, name, type, content, ttl, priority, created_at, updated_at FROM records WHERE id = ?",
		id,
	).Scan(
		&record.ID, &record.Zone, &record.Name, &record.Type, &record.Content,
		&record.TTL, &record.Priority, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Record not found
		}
		return nil, err
	}

	return &record, nil
}

// DeleteRecord deletes a DNS record
func (m *MariaDBClient) DeleteRecord(id int64) error {
	_, err := m.db.Exec("DELETE FROM records WHERE id = ?", id)
	return err
}

// GetAllZones retrieves all zones from the database
func (m *MariaDBClient) GetAllZones() ([]models.Zone, error) {
	rows, err := m.db.Query("SELECT id, name, created_at, updated_at FROM zones")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []models.Zone
	for rows.Next() {
		var zone models.Zone
		err := rows.Scan(
			&zone.ID, &zone.Name, &zone.CreatedAt, &zone.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return zones, nil
}
