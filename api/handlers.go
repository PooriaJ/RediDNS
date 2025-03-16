package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PooriaJ/RediDNS/models"
	"github.com/gorilla/mux"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// healthCheckHandler handles health check requests
func (a *APIServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		},
	})
}

// statsHandler returns DNS server statistics
func (a *APIServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	// This would be implemented to fetch stats from the DNS server
	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"status": "Statistics would be shown here",
		},
	})
}

// listZonesHandler lists all DNS zones
func (a *APIServer) listZonesHandler(w http.ResponseWriter, r *http.Request) {
	// Get all zones from the database
	zones, err := a.mariadbClient.GetAllZones()
	if err != nil {
		a.logger.Errorf("Error getting zones: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to get zones")
		return
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    zones,
	})
}

// createZoneHandler creates a new DNS zone
func (a *APIServer) createZoneHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		responseError(w, http.StatusBadRequest, "Zone name is required")
		return
	}

	// Check if zone already exists
	existingZone, err := a.mariadbClient.GetZone(req.Name)
	if err != nil {
		a.logger.Errorf("Error checking for existing zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for existing zone")
		return
	}

	if existingZone != nil {
		responseError(w, http.StatusConflict, "Zone already exists")
		return
	}

	// Create the zone
	zone, err := a.mariadbClient.CreateZone(req.Name)
	if err != nil {
		a.logger.Errorf("Error creating zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to create zone")
		return
	}

	// Create default SOA record for the zone
	err = a.createDefaultSOARecord(zone.Name)
	if err != nil {
		a.logger.Errorf("Error creating default SOA record: %v", err)
		// Continue even if SOA creation fails, as the zone was created successfully
	}

	responseJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    zone,
	})
}

// getZoneHandler gets a specific DNS zone
func (a *APIServer) getZoneHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	zone, err := a.mariadbClient.GetZone(name)
	if err != nil {
		a.logger.Errorf("Error getting zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to get zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    zone,
	})
}

// deleteZoneHandler deletes a DNS zone
func (a *APIServer) deleteZoneHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Check if zone exists
	zone, err := a.mariadbClient.GetZone(name)
	if err != nil {
		a.logger.Errorf("Error checking for zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	// Delete the zone
	if err := a.mariadbClient.DeleteZone(name); err != nil {
		a.logger.Errorf("Error deleting zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to delete zone")
		return
	}

	// Invalidate cache for this zone
	ctx := context.Background()
	pattern := fmt.Sprintf("dns:record:%s:*", name)
	keys, _ := a.redisClient.Keys(ctx, pattern)
	if len(keys) > 0 {
		a.redisClient.Del(ctx, keys...)
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"message": "Zone deleted successfully"},
	})
}

// listRecordsHandler lists all records for a zone
func (a *APIServer) listRecordsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneName := vars["zone"]

	// Check if zone exists
	zone, err := a.mariadbClient.GetZone(zoneName)
	if err != nil {
		a.logger.Errorf("Error checking for zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	// Get records for the zone
	records, err := a.mariadbClient.GetRecordsByZone(zoneName)
	if err != nil {
		a.logger.Errorf("Error getting records: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to get records")
		return
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    records,
	})
}

// updateZoneSOASerial updates the SOA record's serial number for a zone
func (a *APIServer) updateZoneSOASerial(zoneName string) error {
	// Get the SOA record for the zone
	soaRecords, err := a.mariadbClient.GetRecordsByNameAndType(zoneName, zoneName, models.TypeSOA)
	if err != nil {
		return fmt.Errorf("failed to get SOA record: %w", err)
	}

	// If no SOA record exists, create one
	if len(soaRecords) == 0 {
		return a.createDefaultSOARecord(zoneName)
	}

	// Get the first SOA record
	soaRecord := soaRecords[0]

	// Parse the SOA record content
	var soaData models.SOARecord
	if err := json.Unmarshal([]byte(soaRecord.Content), &soaData); err != nil {
		return fmt.Errorf("failed to parse SOA record: %w", err)
	}

	// Generate new serial number based on current timestamp
	newSerial := uint32(time.Now().Unix())

	// Update the serial number
	soaData.Serial = newSerial

	// Marshal the updated SOA record data
	soaContent, err := json.Marshal(soaData)
	if err != nil {
		return fmt.Errorf("failed to marshal SOA record: %w", err)
	}

	// Update the record content
	soaRecord.Content = string(soaContent)

	// Update the record in the database
	if err := a.mariadbClient.UpdateRecord(&soaRecord); err != nil {
		return fmt.Errorf("failed to update SOA record: %w", err)
	}

	// Invalidate cache for this record
	ctx := context.Background()

	// Invalidate single record cache
	singleCacheKey := fmt.Sprintf("dns:record:%s:%s:%s", soaRecord.Zone, soaRecord.Name, soaRecord.Type)
	a.redisClient.Del(ctx, singleCacheKey)

	// Invalidate multiple records cache
	multiCacheKey := fmt.Sprintf("dns:records:%s:%s:%s", soaRecord.Zone, soaRecord.Name, soaRecord.Type)
	a.redisClient.Del(ctx, multiCacheKey)

	return nil
}

// createRecordHandler creates a new DNS record
func (a *APIServer) createRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneName := vars["zone"]

	// Check if zone exists
	zone, err := a.mariadbClient.GetZone(zoneName)
	if err != nil {
		a.logger.Errorf("Error checking for zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	// Parse request body
	var record models.Record
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set the zone
	record.Zone = zoneName

	// Validate record
	if record.Name == "" {
		responseError(w, http.StatusBadRequest, "Record name is required")
		return
	}

	// Handle @ symbol for root domain
	if record.Name == "@" {
		record.Name = zoneName
	} else if !strings.Contains(record.Name, ".") {
		// If name doesn't contain a dot, it's a subdomain - append the zone name
		record.Name = record.Name + "." + zoneName
	}

	if record.Type == "" {
		responseError(w, http.StatusBadRequest, "Record type is required")
		return
	}

	if record.Content == "" {
		responseError(w, http.StatusBadRequest, "Record content is required")
		return
	}

	// Set default TTL if not provided
	if record.TTL <= 0 {
		record.TTL = 120 // 2 MIN default
	} else {
		// Validate TTL is one of the allowed values
		validTTLs := []int{5, 10, 30, 60, 90, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000, 1296000, 2592000}
		isValid := false
		for _, ttl := range validTTLs {
			if record.TTL == ttl {
				isValid = true
				break
			}
		}
		if !isValid {
			responseError(w, http.StatusBadRequest, "Invalid TTL value. TTL must be one of: 5, 10, 30, 60, 90, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000, 1296000, 2592000 seconds")
			return
		}
	}

	// Create the record
	if err := a.mariadbClient.CreateRecord(&record); err != nil {
		a.logger.Errorf("Error creating record: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to create record")
		return
	}

	// Update the zone's SOA serial number
	if record.Type != models.TypeSOA { // Don't update SOA when creating an SOA record
		if err := a.updateZoneSOASerial(zoneName); err != nil {
			a.logger.Warnf("Failed to update SOA serial: %v", err)
		}
	}

	// Publish record update event
	ctx := context.Background()
	if err := a.redisClient.PublishRecordUpdate(ctx, &record); err != nil {
		a.logger.Warnf("Failed to publish record update: %v", err)
	}

	responseJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    record,
	})
}

// getRecordHandler gets a specific DNS record
func (a *APIServer) getRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneName := vars["zone"]
	recordIDStr := vars["id"]

	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		responseError(w, http.StatusBadRequest, "Invalid record ID")
		return
	}

	// Implementation would get the record from the database
	// For now, return a placeholder response
	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data: models.Record{
			ID:   recordID,
			Zone: zoneName,
			// Other fields would be populated from the database
		},
	})
}

// updateRecordHandler updates a DNS record
func (a *APIServer) updateRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneName := vars["zone"]
	recordIDStr := vars["id"]

	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		responseError(w, http.StatusBadRequest, "Invalid record ID")
		return
	}

	// Check if zone exists
	zone, err := a.mariadbClient.GetZone(zoneName)
	if err != nil {
		a.logger.Errorf("Error checking for zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	// Parse request body
	var updateData struct {
		Name     string `json:"name"`
		Content  string `json:"content"`
		TTL      int    `json:"ttl"`
		Priority int    `json:"priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the existing record
	record, err := a.mariadbClient.GetRecordByID(recordID)
	if err != nil {
		a.logger.Errorf("Error getting record: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to get record")
		return
	}

	if record == nil {
		responseError(w, http.StatusNotFound, "Record not found")
		return
	}

	// Check if record belongs to the specified zone
	if record.Zone != zoneName {
		responseError(w, http.StatusBadRequest, "Record does not belong to the specified zone")
		return
	}

	// Update record fields
	if updateData.Content != "" {
		record.Content = updateData.Content
	}
	if updateData.TTL > 0 {
		// Validate TTL is one of the allowed values
		validTTLs := []int{5, 10, 30, 60, 90, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000, 1296000, 2592000}
		isValid := false
		for _, ttl := range validTTLs {
			if updateData.TTL == ttl {
				isValid = true
				break
			}
		}
		if !isValid {
			responseError(w, http.StatusBadRequest, "Invalid TTL value. TTL must be one of: 5, 10, 30, 60, 90, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000, 1296000, 2592000 seconds")
			return
		}
		record.TTL = updateData.TTL
	}
	record.Priority = updateData.Priority

	// Update the record in the database
	if err := a.mariadbClient.UpdateRecord(record); err != nil {
		a.logger.Errorf("Error updating record: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to update record")
		return
	}

	// Update the zone's SOA serial number
	if record.Type != models.TypeSOA { // Don't update SOA when updating an SOA record
		if err := a.updateZoneSOASerial(zoneName); err != nil {
			a.logger.Warnf("Failed to update SOA serial: %v", err)
		}
	}

	// Invalidate cache for this record
	ctx := context.Background()

	// Invalidate single record cache
	singleCacheKey := fmt.Sprintf("dns:record:%s:%s:%s", record.Zone, record.Name, record.Type)
	a.redisClient.Del(ctx, singleCacheKey)

	// Invalidate multiple records cache
	multiCacheKey := fmt.Sprintf("dns:records:%s:%s:%s", record.Zone, record.Name, record.Type)
	a.redisClient.Del(ctx, multiCacheKey)

	// Publish record update event
	if err := a.redisClient.PublishRecordUpdate(ctx, record); err != nil {
		a.logger.Warnf("Failed to publish record update: %v", err)
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    record,
	})
}

// deleteRecordHandler deletes a DNS record
func (a *APIServer) deleteRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneName := vars["zone"]
	recordIDStr := vars["id"]

	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		responseError(w, http.StatusBadRequest, "Invalid record ID")
		return
	}

	// Check if zone exists
	zone, err := a.mariadbClient.GetZone(zoneName)
	if err != nil {
		a.logger.Errorf("Error checking for zone: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to check for zone")
		return
	}

	if zone == nil {
		responseError(w, http.StatusNotFound, "Zone not found")
		return
	}

	// Get the record before deleting it (to know its name and type for cache invalidation)
	record, err := a.mariadbClient.GetRecordByID(recordID)
	if err != nil {
		a.logger.Errorf("Error getting record: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to get record")
		return
	}

	if record == nil {
		responseError(w, http.StatusNotFound, "Record not found")
		return
	}

	// Delete the record
	if err := a.mariadbClient.DeleteRecord(recordID); err != nil {
		a.logger.Errorf("Error deleting record: %v", err)
		responseError(w, http.StatusInternalServerError, "Failed to delete record")
		return
	}

	// Update the zone's SOA serial number
	if record.Type != models.TypeSOA { // Don't update SOA when deleting an SOA record
		if err := a.updateZoneSOASerial(zoneName); err != nil {
			a.logger.Warnf("Failed to update SOA serial: %v", err)
		}
	}

	// Invalidate cache for this record
	ctx := context.Background()

	// Invalidate single record cache
	singleCacheKey := fmt.Sprintf("dns:record:%s:%s:%s", record.Zone, record.Name, record.Type)
	a.redisClient.Del(ctx, singleCacheKey)

	// Invalidate multiple records cache
	multiCacheKey := fmt.Sprintf("dns:records:%s:%s:%s", record.Zone, record.Name, record.Type)
	a.redisClient.Del(ctx, multiCacheKey)

	// Publish record update event for cache invalidation across instances
	if err := a.redisClient.PublishRecordUpdate(ctx, record); err != nil {
		a.logger.Warnf("Failed to publish record update: %v", err)
	}

	responseJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"message": "Record deleted successfully",
		},
	})
}

// Helper functions for API responses

// responseJSON sends a JSON response
func responseJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// createDefaultSOARecord creates a default SOA record for a new zone
func (a *APIServer) createDefaultSOARecord(zoneName string) error {
	// Get SOA configuration from config
	primaryNameserver := a.config.DNS.SOA.PrimaryNameserver
	mailAddress := a.config.DNS.SOA.MailAddress
	refresh := uint32(a.config.DNS.SOA.Refresh)
	retry := uint32(a.config.DNS.SOA.Retry)
	expire := uint32(a.config.DNS.SOA.Expire)
	minimum := uint32(a.config.DNS.SOA.Minimum)

	// Generate serial number based on current timestamp
	serial := uint32(time.Now().Unix())

	// Create SOA record data
	soaData := models.SOARecord{
		Mname:   primaryNameserver,
		Rname:   mailAddress,
		Serial:  serial,
		Refresh: refresh,
		Retry:   retry,
		Expire:  expire,
		Minimum: minimum,
	}

	// Marshal SOA record to JSON for storage
	soaContent, err := json.Marshal(soaData)
	if err != nil {
		return fmt.Errorf("failed to marshal SOA record: %w", err)
	}

	// Create the record
	record := &models.Record{
		Zone:     zoneName,
		Name:     zoneName, // SOA record is at the apex of the zone
		Type:     models.TypeSOA,
		Content:  string(soaContent),
		TTL:      86400, // 24 hours
		Priority: 0,
	}

	// Store in database
	if err := a.mariadbClient.CreateRecord(record); err != nil {
		return fmt.Errorf("failed to create SOA record: %w", err)
	}

	return nil
}

// responseError sends an error response
func responseError(w http.ResponseWriter, status int, message string) {
	responseJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}
