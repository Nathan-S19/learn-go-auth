package main

import (
	"context"
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

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	username := "exampleUser"

	token, err := GenerateJWT(username)
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, token)))
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(userContextKey).(string)
	w.Write([]byte(fmt.Sprintf("Hello, %s!", username)))
}
