package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

const (
	dbFile = "data/ip_ranges.db"
)

var (
	dataURL string
	db      *sql.DB
)

type IPRange struct {
	StartIP       string `json:"start_ip"`
	EndIP         string `json:"end_ip"`
	Country       string `json:"country"`
	CountryName   string `json:"country_name"`
	Continent     string `json:"continent"`
	ContinentName string `json:"continent_name"`
}

type IPInfo struct {
	IP            string `json:"ip"`
	CountryName   string `json:"country_name"`
	ContinentName string `json:"continent_name"`
}

func init() {
	dataURL = os.Getenv("IP_DATA_URL")
	if dataURL == "" {
		log.Fatal("IP_DATA_URL environment variable is not set")
	}
}

func main() {
	err := os.MkdirAll(filepath.Dir(dbFile), 0755)
	if err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createTable()
	if err != nil {
		log.Fatal(err)
	}

	err = updateIPRangesIfNeeded()
	if err != nil {
		log.Printf("Error during initial data load: %v", err)
	}

	c := cron.New(cron.WithLocation(time.UTC))
	_, err = c.AddFunc("30 0 * * *", func() {
		log.Println("Starting scheduled update check...")
		err := updateIPRangesIfNeeded()
		if err != nil {
			log.Printf("Error during scheduled update: %v", err)
		}
		log.Println("Scheduled update check completed.")
	})
	if err != nil {
		log.Fatal(err)
	}
	c.Start()

	r := mux.NewRouter()
	r.HandleFunc("/", autoDetectHandler).Methods("GET")
	r.HandleFunc("/lookup/{ip}", lookupHandler).Methods("GET")

	log.Println("Server is running on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func createTable() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ip_ranges (
			start_ip BLOB,
			end_ip BLOB,
			country_name TEXT,
			continent_name TEXT,
			is_ipv6 BOOLEAN
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ip_ranges table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create metadata table: %v", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ip_range ON ip_ranges (start_ip, end_ip, is_ipv6)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	return nil
}

func updateIPRangesIfNeeded() error {
	lastUpdate, err := getLastUpdateDate()
	if err != nil {
		return fmt.Errorf("failed to get last update date: %v", err)
	}

	currentDate := time.Now().UTC().Format("2006-01-02")
	if lastUpdate == currentDate {
		log.Println("Data is up to date. Skipping update.")
		return nil
	}

	log.Println("Updating IP ranges data...")
	err = updateIPRanges()
	if err != nil {
		return fmt.Errorf("failed to update IP ranges: %v", err)
	}

	err = setLastUpdateDate(currentDate)
	if err != nil {
		return fmt.Errorf("failed to set last update date: %v", err)
	}

	return nil
}

func updateIPRanges() error {
	log.Println("Downloading new IP ranges data...")
	resp, err := http.Get(dataURL)
	if err != nil {
		return fmt.Errorf("failed to download data: %v", err)
	}
	defer resp.Body.Close()

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tmpFile, err := os.CreateTemp("", "ip_ranges_*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, gzReader)
	if err != nil {
		return fmt.Errorf("failed to write to temp file: %v", err)
	}

	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek temp file: %v", err)
	}

	log.Println("Loading new data into database...")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM ip_ranges")
	if err != nil {
		return fmt.Errorf("failed to clear existing data: %v", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO ip_ranges (start_ip, end_ip, country_name, continent_name, is_ipv6)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	decoder := json.NewDecoder(tmpFile)
	for decoder.More() {
		var ipRange IPRange
		if err := decoder.Decode(&ipRange); err != nil {
			return fmt.Errorf("failed to decode JSON: %v", err)
		}

		startIP := net.ParseIP(ipRange.StartIP)
		endIP := net.ParseIP(ipRange.EndIP)
		if startIP == nil || endIP == nil {
			log.Printf("Warning: Invalid IP range %s - %s", ipRange.StartIP, ipRange.EndIP)
			continue
		}

		isIPv6 := startIP.To4() == nil
		var startIPBytes, endIPBytes []byte
		if isIPv6 {
			startIPBytes = startIP.To16()
			endIPBytes = endIP.To16()
		} else {
			startIPBytes = startIP.To4()
			endIPBytes = endIP.To4()
		}

		_, err = stmt.Exec(startIPBytes, endIPBytes, ipRange.CountryName, ipRange.ContinentName, isIPv6)
		if err != nil {
			return fmt.Errorf("failed to insert data: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Println("Database updated successfully.")
	return nil
}

func getLastUpdateDate() (string, error) {
	var lastUpdateStr string
	err := db.QueryRow("SELECT value FROM metadata WHERE key = 'last_update_date'").Scan(&lastUpdateStr)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if no update has been performed yet
	} else if err != nil {
		return "", err
	}
	return lastUpdateStr, nil
}

func setLastUpdateDate(date string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES ('last_update_date', ?)", date)
	return err
}

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ipStr := vars["ip"]

	info, err := lookupIP(ipStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(info)
}

func autoDetectHandler(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)

	info, err := lookupIP(ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(info)
}

func lookupIP(ipStr string) (*IPInfo, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("Invalid IP address")
	}

	isIPv6 := ip.To4() == nil
	var ipBytes []byte
	if isIPv6 {
		ipBytes = ip.To16()
	} else {
		ipBytes = ip.To4()
	}

	var info IPInfo
	err := db.QueryRow(`
		SELECT ?, country_name, continent_name
		FROM ip_ranges
		WHERE ? BETWEEN start_ip AND end_ip AND is_ipv6 = ?
		LIMIT 1
	`, ipStr, ipBytes, isIPv6).Scan(&info.IP, &info.CountryName, &info.ContinentName)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("IP not found in any range")
	} else if err != nil {
		log.Println("Database query error:", err)
		return nil, fmt.Errorf("Internal server error")
	}

	return &info, nil
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	return ip
}
