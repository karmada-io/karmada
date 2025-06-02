/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/05/27
 * @Desc    : 全局时钟服务
 */

package clock

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TimeResponse time response
type TimeResponse struct {
	UTC         string `json:"utc"`
	Local       string `json:"local"`
	Timestamp   int64  `json:"timestamp"`
	Timezone    string `json:"timezone"`
	Version     string `json:"version"`
	ProcessTime string `json:"process_time"`
}

// ClientInfo client info
type ClientInfo struct {
	ClientID    string `json:"client_id"`
	Region      string `json:"region"`
	Timezone    string `json:"timezone"`
	LastSyncUTC string `json:"last_sync_utc"`
}

// TimeServer time server
type TimeServer struct {
	ctx          context.Context
	port         string
	enableHTTPS  bool
	certFile     string
	mu           sync.RWMutex
	keyFile      string
	LogicalClock int64
	clients      map[string]*ClientInfo
}

// NewTimeServer create time server
func NewTimeServer(ctx context.Context, port string) *TimeServer {
	return &TimeServer{
		ctx:     ctx,
		port:    port,
		clients: make(map[string]*ClientInfo),
		mu:      sync.RWMutex{},
	}
}

// EnableHTTPS enable HTTPS
func (ts *TimeServer) EnableHTTPS(certFile, keyFile string) {
	ts.enableHTTPS = true
	ts.certFile = certFile
	ts.keyFile = keyFile
}

// handleTimeSync handle time sync request
func (ts *TimeServer) handleTimeSync(w http.ResponseWriter, r *http.Request) {
	// get current UTC time
	now := time.Now()

	// get client info
	clientID := r.Header.Get("Client-ID")
	region := r.Header.Get("Region")
	timezone := r.Header.Get("Timezone")

	// update client info
	if clientID != "" {
		ts.updateClientInfo(clientID, region, timezone)
	}

	var localTime time.Time
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			localTime = now.In(loc)
		} else {
			localTime = now // 降级到服务器本地时间
		}
	} else {
		localTime = now
	}

	// build response
	response := TimeResponse{
		UTC:         now.UTC().Format("2006-01-02T15:04:05.000Z"),
		Local:       localTime.Format("2006-01-02T15:04:05.000-07:00"),
		Timestamp:   now.UnixMilli(),
		Timezone:    localTime.Location().String(),
		Version:     "1.0.0",
		ProcessTime: time.Since(now).String(),
	}

	// log
	logrus.Printf("Time sync request from client=%s region=%s tz=%s client=%s",
		clientID, region, timezone, r.RemoteAddr)

	// send response
	json.NewEncoder(w).Encode(response)
}

// handleClientStatus handle client status query
func (ts *TimeServer) handleClientStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type StatusResponse struct {
		TotalClients int                    `json:"total_clients"`
		Clients      map[string]*ClientInfo `json:"clients"`
		ServerTime   string                 `json:"server_time"`
	}

	response := StatusResponse{
		TotalClients: len(ts.clients),
		Clients:      ts.clients,
		ServerTime:   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	}

	json.NewEncoder(w).Encode(response)
}

// updateClusterInfo 更新集群信息
func (ts *TimeServer) updateClientInfo(clientID, region, timezone string) {
	ts.clients[clientID] = &ClientInfo{
		ClientID:    clientID,
		Region:      region,
		Timezone:    timezone,
		LastSyncUTC: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	}
}

// handleHealth 健康检查
func (ts *TimeServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"uptime": "running",
	}

	json.NewEncoder(w).Encode(response)
}

func (ts *TimeServer) GetLogicalClock() int64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.LogicalClock
}

// SyncLogicalClock : sync logical clock (for distributed time service)
func (ts *TimeServer) handleLogicalClock(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	remoteClock := r.Header.Get("Logical-Clock")
	if remoteClock != "" {
		remoteClockInt, err := strconv.ParseInt(remoteClock, 10, 64)
		if err != nil {
			logrus.Printf("invalid logical clock: %v", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "invalid logical clock",
			})
			return
		}
		ts.LogicalClock = remoteClockInt
	}

	// Lamport clock synchronization algorithm
	ts.LogicalClock++

	json.NewEncoder(w).Encode(ts.LogicalClock)
}

// Start start server
func (ts *TimeServer) Start() error {
	mux := http.NewServeMux()

	// register routes
	mux.HandleFunc("/time/sync", ts.handleTimeSync)
	mux.HandleFunc("/time/logical", ts.handleLogicalClock)
	mux.HandleFunc("/client/status", ts.handleClientStatus)
	mux.HandleFunc("/health", ts.handleHealth)

	// add middleware
	handler := ts.loggingMiddleware(mux)

	server := &http.Server{
		Addr:           ts.port,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	logrus.Printf("🚀 Time Server starting on port %s", ts.port)
	logrus.Printf("📡 Endpoints:")
	logrus.Printf("   POST   /time/sync      - time sync")
	logrus.Printf("   GET    /client/status  - client status")
	logrus.Printf("   GET    /health         - health check")

	if ts.enableHTTPS {
		logrus.Printf("🔒 HTTPS enabled")
		return server.ListenAndServeTLS(ts.certFile, ts.keyFile)
	} else {
		logrus.Printf("⚠️  HTTP mode (consider enabling HTTPS for production)")
		return server.ListenAndServe()
	}
}

// loggingMiddleware logging middleware
func (ts *TimeServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// call next handler
		next.ServeHTTP(w, r)

		// log request
		duration := time.Since(start)
		logrus.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, duration)
	})
}

// type GlobalTime struct {
// 	Timestamp    int64  `json:"timestamp"`     // UTC timestamp
// 	LogicalClock int64  `json:"logical_clock"` // logical clock
// 	Epoch        int64  `json:"epoch"`         // base time
// 	Lease        int64  `json:"lease"`         // lease time
// 	ServerID     string `json:"server_id"`     // server id
// }

// type TimeRequest struct {
// 	ClientID string `json:"client_id,omitempty"` // client id (optional)
// 	Region   string `json:"region,omitempty"`    // region info (optional)
// }

// type TimeService struct {
// 	ctx          context.Context
// 	mu           sync.RWMutex
// 	logicalClock int64
// 	baseTime     time.Time
// 	serverID     string
// 	lease        int64
// }

// var ts *TimeService

// NewTimeService : create a time service
// func NewTimeService(ctx context.Context, serverID string) *TimeService {
// 	if ts != nil {
// 		return ts
// 	}

// 	ts = &TimeService{
// 		ctx:          ctx,
// 		logicalClock: 0,
// 		baseTime:     time.Now().UTC(),
// 		serverID:     serverID,
// 	}
// 	return ts
// }

// func (pts *TimeService) Lease(lease int64) {
// 	if lease <= 0 {
// 		lease = 1000
// 	}

// 	ticker := time.NewTicker(time.Millisecond * time.Duration(lease))
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-pts.ctx.Done():
// 			return
// 		case <-ticker.C:
// 			pts.mu.Lock()
// 			pts.lease = pts.lease + 1
// 			pts.mu.Unlock()
// 		}
// 	}
// }

// // TimeHandler : HTTP handler for time service
// func (pts *TimeService) TimeHandler(w http.ResponseWriter, r *http.Request) {
// 	var req TimeRequest

// 	// parse request (optional)
// 	if r.Method == "POST" {
// 		json.NewDecoder(r.Body).Decode(&req)
// 	}

// 	globalTime := pts.GetGlobalTime()

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(globalTime)
// }

// // HealthHandler : HTTP handler for health check
// func (pts *TimeService) HealthHandler(w http.ResponseWriter, r *http.Request) {
// 	health := map[string]any{
// 		"status":        "healthy",
// 		"server_id":     pts.serverID,
// 		"current_time":  time.Now().UTC(),
// 		"logical_clock": pts.GetLogicalClock(),
// 		"lease":         pts.lease,
// 		"uptime":        time.Since(pts.baseTime),
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(health)
// }

// // GetGlobalTime : get global time
// func (pts *TimeService) GetGlobalTime() GlobalTime {
// 	pts.mu.Lock()
// 	defer pts.mu.Unlock()

// 	pts.logicalClock++

// 	return GlobalTime{
// 		Timestamp:    time.Now().UTC().UnixMilli(),
// 		LogicalClock: pts.logicalClock,
// 		Epoch:        pts.baseTime.UnixMilli(),
// 		ServerID:     pts.serverID,
// 		Lease:        pts.lease,
// 	}
// }

// GetLogicalClock : get current logical clock
// func (pts *TimeService) GetLogicalClock() int64 {
// 	pts.mu.RLock()
// 	defer pts.mu.RUnlock()
// 	return pts.logicalClock
// }

// // SyncLogicalClock : sync logical clock (for distributed time service)
// func (pts *TimeService) SyncLogicalClock(remoteClock int64) {
// 	pts.mu.Lock()
// 	defer pts.mu.Unlock()

// 	// Lamport clock synchronization algorithm
// 	if remoteClock > pts.logicalClock {
// 		pts.logicalClock = remoteClock
// 	}
// 	pts.logicalClock++
// }
