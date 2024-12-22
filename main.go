package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type FingerPrint struct {
	ID      int    `json:"id"`
	PIN     string `json:"pin"`
	AttLog  string `json:"attlog"`
	CloudID string `json:"cloud_id"`
}

var db *sql.DB

func connectDB() {
	var err error
	dsn := os.Getenv("DB_DSN")
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Error pinging the database: %v", err)
	}
	fmt.Println("Connected to the database!")
}

func verifyKey(r *http.Request) bool {
	apiKey := r.Header.Get("X-API-KEY")
	expectedKey := os.Getenv("API_KEY")
	return apiKey == expectedKey
}

func fetchFingerPrints(w http.ResponseWriter, r *http.Request) {
	if !verifyKey(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := db.Query("SELECT id, pin, attlog, cloud_id FROM tb_fps WHERE is_fetched = 0 LIMIT 1000")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var fingerprints []FingerPrint
	for rows.Next() {
		var fp FingerPrint
		if err := rows.Scan(&fp.ID, &fp.PIN, &fp.AttLog, &fp.CloudID); err != nil {
			http.Error(w, "Error scanning data", http.StatusInternalServerError)
			return
		}
		fingerprints = append(fingerprints, fp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fingerprints)
}

func updateFetchedStatus(w http.ResponseWriter, r *http.Request) {
	if !verifyKey(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var ids []int
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	query := "UPDATE tb_fps SET is_fetched = 1 WHERE id IN (" + strings.Join(intSliceToStringSlice(ids), ",") + ")"
	_, err := db.Exec(query)
	if err != nil {
		http.Error(w, "Database update error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Update successful"))
}

func intSliceToStringSlice(ids []int) []string {
	var strIDs []string
	for _, id := range ids {
		strIDs = append(strIDs, strconv.Itoa(id))
	}
	return strIDs
}

func main() {
	connectDB()
	defer db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/fetch", fetchFingerPrints).Methods("GET")
	r.HandleFunc("/update", updateFetchedStatus).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
