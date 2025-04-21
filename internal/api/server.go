package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chashtager/orchestron/internal/config"
	"github.com/chashtager/orchestron/internal/metrics"
	"github.com/chashtager/orchestron/internal/models"
	"github.com/chashtager/orchestron/internal/service"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// Server handles HTTP requests
type Server struct {
	config         *config.APIConfig
	eventProcessor service.EventProcessor
	rateLimiters   map[string]*rate.Limiter
	schemaRepository service.SchemaRepository
}

// NewServer creates a new HTTP server
func NewServer(config *config.APIConfig, eventProcessor service.EventProcessor, schemaRepository service.SchemaRepository) *Server {
	server := &Server{
		config:         config,
		eventProcessor: eventProcessor,
		rateLimiters:   make(map[string]*rate.Limiter),
		schemaRepository: schemaRepository,
	}

	// Initialize rate limiters for each API key
	for _, key := range config.APIKeys {
		server.rateLimiters[key] = rate.NewLimiter(
			rate.Limit(config.RateLimit),
			config.RateLimitBurst,
		)
	}

	return server
}

// Router sets up the HTTP routes
func (s *Server) Router() *mux.Router {
	r := mux.NewRouter()
	
	// Health check endpoint
	r.HandleFunc("/health", s.healthCheckHandler).Methods("GET")

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())
	
	// API routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(s.authMiddleware)
	api.Use(s.rateLimitMiddleware)
	api.Use(s.loggingMiddleware)
	
	// Event endpoints
	events := api.PathPrefix("/events").Subrouter()
	events.HandleFunc("/{eventType}", s.receiveEventHandler).Methods("POST")
	
	// Schema endpoints
	schemas := api.PathPrefix("/schemas").Subrouter()
	schemas.HandleFunc("", s.registerSchemaHandler).Methods("POST")
	
	return r
}

// healthCheckHandler handles health check requests
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}


// receiveEventHandler handles incoming webhook events
func (s *Server) receiveEventHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventType := vars["eventType"]
	
	// Validate event type
	if !s.isValidEventType(eventType) {
		http.Error(w, "Invalid event type", http.StatusBadRequest)
		return
	}

	metrics.EventsReceived.WithLabelValues(eventType).Inc()

	// Generate event ID
	eventID := uuid.New().String()
	
	// Parse payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}
	
	// Extract relevant headers
	metadata := extractRelevantHeaders(r.Header)
	
	// Create event
	event := &models.Event{
		ID:        eventID,
		Type:      eventType,
		Payload:   payload,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
	
	// Process event asynchronously
	go func() {
		// Create a new background context for async processing
		ctx := context.Background()
		if err := s.eventProcessor.ProcessEvent(ctx, event); err != nil {
			log.Printf("Error processing event %s: %v", eventID, err)
		}
	}()
	
	// Return accepted response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"eventId": eventID,
		"status":  "accepted",
	})
}

// authMiddleware handles authentication
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from header
		apiKey := r.Header.Get("X-API-Key")
		
		// Check if API key is valid
		if apiKey == "" || !s.isValidAPIKey(apiKey) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// Add API key to context
		ctx := context.WithValue(r.Context(), models.ApiKeyContextKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rateLimitMiddleware handles rate limiting
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from context
		apiKey, ok := r.Context().Value(models.ApiKeyContextKey).(string)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get rate limiter for API key
		limiter, ok := s.rateLimiters[apiKey]
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check rate limit
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isValidAPIKey checks if the API key is valid
func (s *Server) isValidAPIKey(apiKey string) bool {
	_, ok := s.rateLimiters[apiKey]
	return ok
}

// isValidEventType checks if the event type is valid
func (s *Server) isValidEventType(eventType string) bool {
	// In a real implementation, this would check against a list of allowed event types
	return len(eventType) > 0 && len(eventType) <= 100 && !strings.ContainsAny(eventType, " \t\n\r")
}

// loggingMiddleware logs incoming requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a custom response writer to capture status code
		lrw := newLoggingResponseWriter(w)
		
		// Call the next handler
		next.ServeHTTP(lrw, r)
		
		// Log the request
		log.Printf(
			"%s %s %d %s %s %s",
			r.Method,
			r.RequestURI,
			lrw.statusCode,
			time.Since(start),
			r.RemoteAddr,
			r.Header.Get("X-API-Key"),
		)
	})
}

// loggingResponseWriter is a custom response writer that captures status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newLoggingResponseWriter creates a new logging response writer
func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// extractRelevantHeaders extracts relevant headers from the request
func extractRelevantHeaders(headers http.Header) map[string]string {
	metadata := make(map[string]string)
	
	for name, values := range headers {
		if len(values) > 0 && (name == "Content-Type" || name == "User-Agent" || name == "X-Request-ID") {
			metadata[name] = values[0]
		}
	}
	
	return metadata
}

// registerSchemaHandler handles schema registration
func (s *Server) registerSchemaHandler(w http.ResponseWriter, r *http.Request) {
	var schema models.EventSchema
	if err := json.NewDecoder(r.Body).Decode(&schema); err != nil {
		http.Error(w, fmt.Sprintf("Invalid schema: %v", err), http.StatusBadRequest)
		return
	}

	// Save schema
	if err := s.schemaRepository.Save(r.Context(), &schema); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save schema: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
	})
}
