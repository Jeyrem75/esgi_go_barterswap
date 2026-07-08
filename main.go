package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func InitDB() *sql.DB {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return db
}

func main() {
	db := InitDB()
	defer db.Close()

	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	RegisterServiceRoutes(mux, db)
	RegisterExchangeRoutes(mux, db)

	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root.Handle("/", AuthMiddleware(mux))

	handler := LoggingMiddleware(
		RecoveryMiddleware(
			CORSMiddleware(
				TimeoutMiddleware(5 * time.Second)(root))))

	log.Println("BarterSwap API sur :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}