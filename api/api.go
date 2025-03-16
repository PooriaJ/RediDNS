package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/PooriaJ/RediDNS/config"
	"github.com/PooriaJ/RediDNS/db"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// APIServer represents the API server for DNS management
type APIServer struct {
	router        *mux.Router
	redisClient   *db.RedisClient
	mariadbClient *db.MariaDBClient
	logger        *logrus.Logger
	config        *config.Config
	server        *http.Server
}

// NewAPIServer creates a new API server
func NewAPIServer(cfg *config.Config, redisClient *db.RedisClient, mariadbClient *db.MariaDBClient, logger *logrus.Logger) *APIServer {
	router := mux.NewRouter()

	api := &APIServer{
		config:        cfg,
		redisClient:   redisClient,
		mariadbClient: mariadbClient,
		logger:        logger,
		router:        router,
	}

	// Setup routes
	api.setupRoutes()

	return api
}

// Start starts the API server
func (a *APIServer) Start() error {
	addr := fmt.Sprintf("%s:%d", a.config.API.Address, a.config.API.Port)
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	a.logger.Infof("Starting API server on %s", addr)
	return a.server.ListenAndServe()
}

// Stop stops the API server
func (a *APIServer) Stop() error {
	if a.server != nil {
		a.logger.Info("Shutting down API server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(ctx)
	}
	return nil
}

// setupRoutes sets up the API routes
func (a *APIServer) setupRoutes() {
	// API version prefix
	v1 := a.router.PathPrefix("/api/v1").Subrouter()

	// Health check
	v1.HandleFunc("/health", a.healthCheckHandler).Methods("GET")

	// Zones
	v1.HandleFunc("/zones", a.listZonesHandler).Methods("GET")
	v1.HandleFunc("/zones", a.createZoneHandler).Methods("POST")
	v1.HandleFunc("/zones/{name}", a.getZoneHandler).Methods("GET")
	v1.HandleFunc("/zones/{name}", a.deleteZoneHandler).Methods("DELETE")

	// Records
	v1.HandleFunc("/zones/{zone}/records", a.listRecordsHandler).Methods("GET")
	v1.HandleFunc("/zones/{zone}/records", a.createRecordHandler).Methods("POST")
	v1.HandleFunc("/zones/{zone}/records/{id}", a.getRecordHandler).Methods("GET")
	v1.HandleFunc("/zones/{zone}/records/{id}", a.updateRecordHandler).Methods("PUT")
	v1.HandleFunc("/zones/{zone}/records/{id}", a.deleteRecordHandler).Methods("DELETE")

	// Stats
	v1.HandleFunc("/stats", a.statsHandler).Methods("GET")

	// Add middleware
	a.router.Use(a.loggingMiddleware)
}

// loggingMiddleware logs all requests
func (a *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the next handler
		next.ServeHTTP(w, r)

		a.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
			"duration":    time.Since(start),
		}).Info("API request")
	})
}
