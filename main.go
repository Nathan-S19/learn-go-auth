package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "duration for shutdown")
	flag.Parse()

	InitializeDB()
	defer CloseDB()

	r := mux.NewRouter()

	// Public Routes
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/login", LoginHandler).Methods("POST")
	r.HandleFunc("/register", RegisterHandler).Methods("POST")
	r.HandleFunc("/refresh-token", RefreshTokenHandler).Methods("POST")

	// Protected Routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(JWTMiddleware)
	api.HandleFunc("/hello", HelloHandler).Methods("GET")

	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var user struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	hashedPassword := hashPassword(user.Password)

	_, err = DB.Exec(`INSERT INTO users (username, password_hash, email) VALUES ($1, $2, $3)`,
		user.Username, hashedPassword, user.Email)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User created successfully"))

}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var user struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var storedHash string
	err = DB.QueryRow(`SELECT password_hash FROM users WHERE username = $1`, user.Username).Scan(&storedHash)
	if err == sql.ErrNoRows || hashPassword(user.Password) != storedHash {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	token, refreshToken, err := GenerateJWT(user.Username)
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"token":"%s", "refresh": "%s"}`, token, refreshToken)))
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(userContextKey).(string)
	w.Write([]byte(fmt.Sprintf("Hello, %s!", username)))
}

func RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Extract the username associated with the refresh token
	username, err := getUsernameFromRefreshToken(request.RefreshToken)
	if err != nil || username == "" {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Validate the refresh token
	ctx := r.Context()
	isValid, err := validateRefreshToken(ctx, username, request.RefreshToken)
	if err != nil || !isValid {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Generate a new JWT token
	newToken, _, err := GenerateJWT(username)
	if err != nil {
		http.Error(w, "Could not generate new token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"token":"%s"}`, newToken)))
}

// getUsernameFromRefreshToken retrieves the username associated with the refresh token
func getUsernameFromRefreshToken(refreshToken string) (string, error) {
	var username string
	err := DB.QueryRow(`
		SELECT u.username FROM users u 
		JOIN refresh_tokens rt ON rt.user_id = u.id 
		WHERE rt.token = $1 AND rt.revoked = FALSE 
		AND rt.expires_at > CURRENT_TIMESTAMP
	`, refreshToken).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}
