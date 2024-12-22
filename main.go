package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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

func respondWithJSON(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func fetchFingerPrints(w http.ResponseWriter, r *http.Request) {
	if !verifyKey(r) {
		respondWithJSON(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	rows, err := db.Query("SELECT id, pin, attlog, cloud_id FROM tb_fps WHERE is_fetched = 0 LIMIT 1000")
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, "Database error", nil)
		return
	}
	defer rows.Close()

	var fingerprints []FingerPrint
	for rows.Next() {
		var fp FingerPrint
		if err := rows.Scan(&fp.ID, &fp.PIN, &fp.AttLog, &fp.CloudID); err != nil {
			respondWithJSON(w, http.StatusInternalServerError, "Error scanning data", nil)
			return
		}
		fingerprints = append(fingerprints, fp)
	}

	respondWithJSON(w, http.StatusOK, "Fingerprints fetched successfully", fingerprints)
}

func updateFetchedStatus(w http.ResponseWriter, r *http.Request) {
	if !verifyKey(r) {
		respondWithJSON(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Decode the JSON body into an object containing `ids`
	var requestBody struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		respondWithJSON(w, http.StatusBadRequest, "Invalid JSON input", nil)
		return
	}

	if len(requestBody.IDs) == 0 {
		respondWithJSON(w, http.StatusBadRequest, "No IDs provided", nil)
		return
	}

	// Use a transaction to update records securely
	tx, err := db.Begin()
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, "Failed to begin transaction", nil)
		log.Printf("Transaction begin error: %v", err)
		return
	}

	// Prepare the statement
	stmt, err := tx.Prepare("UPDATE tb_fps SET is_fetched = 1 WHERE id = ?")
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, "Failed to prepare statement", nil)
		log.Printf("Statement prepare error: %v", err)
		_ = tx.Rollback()
		return
	}
	defer stmt.Close()

	// Execute the update for each ID
	for _, id := range requestBody.IDs {
		if _, err := stmt.Exec(id); err != nil {
			respondWithJSON(w, http.StatusInternalServerError, "Failed to update records", nil)
			log.Printf("Update error for ID %d: %v", id, err)
			_ = tx.Rollback()
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		respondWithJSON(w, http.StatusInternalServerError, "Failed to commit transaction", nil)
		log.Printf("Transaction commit error: %v", err)
		return
	}

	respondWithJSON(w, http.StatusOK, "Update successful", nil)
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
