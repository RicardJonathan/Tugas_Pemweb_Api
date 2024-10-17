package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var db *sql.DB

// Kredensial dasar untuk otentikasi
const (
	username = "admin"
	password = "password"
)

func main() {
	var err error

	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error memuat file .env: %s", err)
	}

	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")

	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(localhost:3306)/movies_db", dbUsername, dbPassword))
	if err != nil {
		log.Fatalf("Gagal terhubung ke MySQL: %s", err)
	}
	defer db.Close()

	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/movies", basicAuth(moviesHandler))     // "Melindungi moviesHandler dengan Autentikasi Dasar."
	http.HandleFunc("/movies/", basicAuth(movieByIDHandler)) // "Melindungi movieByIDHandler dengan Autentikasi Dasar."

	fmt.Println("Server Berjalan Di http://localhost:8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatalf("Kesalahan saat memulai server: %s", err)
	}
}

// Middleware Autentikasi Dasar
func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Memeriksa header Authorization
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "Tidak Berwenang", http.StatusUnauthorized)
			return
		}

		// Memisahkan header menjadi jenis dan kredensial
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Basic" {
			http.Error(w, "Tidak Berwenang", http.StatusUnauthorized)
			return
		}

		// Mendekode kredensial
		credentials, err := decodeBasicAuth(parts[1])
		if err != nil || credentials["username"] != username || credentials["password"] != password {
			http.Error(w, "Tidak Berwenang", http.StatusUnauthorized)
			return
		}

		// Memanggil handler berikutnya
		next(w, r)
	}
}

// Mendekode kredensial Basic Auth
func decodeBasicAuth(encoded string) (map[string]string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("autentikasi tidak valid")
	}
	return map[string]string{"username": parts[0], "password": parts[1]}, nil
}

// Handler status untuk memeriksa status API
func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "API berjalan dan siap digunakan!",
	})
}

// Handler Movies untuk POST (Buat) dan GET (Baca semua)
func moviesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Baca semua film
		getAllMovies(w, r)
	case http.MethodPost:
		// Buat film baru
		createMovie(w, r)
	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}

// Handler Movie berdasarkan ID untuk GET (Baca berdasarkan ID), PUT (Perbarui), dan DELETE (Hapus)
func movieByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := r.URL.Path[len("/movies/"):]

	if idStr == "" {
		http.Error(w, "ID film diperlukan", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID film tidak valid", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Baca film berdasarkan ID
		getMovieByID(w, r, id)
	case http.MethodPut:
		// Perbarui film berdasarkan ID
		updateMovie(w, r, id)
	case http.MethodDelete:
		// Hapus film berdasarkan ID
		deleteMovie(w, r, id)
	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}

// Baca semua film (GET)
func getAllMovies(w http.ResponseWriter, _ *http.Request) {
	var movies []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseYear string  `json:"releaseyear"`
		Genre       string  `json:"genre"`
		Director    string  `json:"director"`
		Rating      float64 `json:"rating"`
		Description string  `json:"description"`
	}

	rows, err := db.Query("SELECT id, title, release_year, genre, director, rating, description FROM movies")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var movie struct {
			ID          int     `json:"id"`
			Title       string  `json:"title"`
			ReleaseYear string  `json:"releaseyear"`
			Genre       string  `json:"genre"`
			Director    string  `json:"director"`
			Rating      float64 `json:"rating"`
			Description string  `json:"description"`
		}
		err := rows.Scan(
			&movie.ID,
			&movie.Title,
			&movie.ReleaseYear,
			&movie.Genre,
			&movie.Director,
			&movie.Rating,
			&movie.Description,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		movies = append(movies, movie)
	}

	json.NewEncoder(w).Encode(movies)
}

// Buat film baru (POST)
func createMovie(w http.ResponseWriter, r *http.Request) {
	var movie struct {
		Title       string  `json:"title"`
		ReleaseYear string  `json:"releaseyear"`
		Genre       string  `json:"genre"`
		Director    string  `json:"director"`
		Rating      float64 `json:"rating"`
		Description string  `json:"description"`
	}

	err := json.NewDecoder(r.Body).Decode(&movie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO movies (title, release_year, genre, director, rating, description) VALUES (?, ?, ?, ?, ?, ?)",
		movie.Title, movie.ReleaseYear, movie.Genre, movie.Director, movie.Rating, movie.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Film berhasil dibuat"})
}

// Baca film berdasarkan ID (GET)
func getMovieByID(w http.ResponseWriter, _ *http.Request, id int) {
	var movie struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseYear string  `json:"releaseyear"`
		Genre       string  `json:"genre"`
		Director    string  `json:"director"`
		Rating      float64 `json:"rating"`
		Description string  `json:"description"`
	}

	err := db.QueryRow("SELECT id, title, release_year, genre, director, rating, description FROM movies WHERE id = ?", id).Scan(
		&movie.ID,
		&movie.Title,
		&movie.ReleaseYear,
		&movie.Genre,
		&movie.Director,
		&movie.Rating,
		&movie.Description,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Film tidak ditemukan", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(movie)
}

// Perbarui film (PUT)
func updateMovie(w http.ResponseWriter, r *http.Request, id int) {
	var movie struct {
		Title       string  `json:"title"`
		ReleaseYear string  `json:"release_year"`
		Genre       string  `json:"genre"`
		Director    string  `json:"director"`
		Rating      float64 `json:"rating"`
		Description string  `json:"description"`
	}

	err := json.NewDecoder(r.Body).Decode(&movie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE movies SET title = ?, release_year = ?, genre = ?, director = ?, rating = ?, description = ? WHERE id = ?",
		movie.Title, movie.ReleaseYear, movie.Genre, movie.Director, movie.Rating, movie.Description, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Film berhasil diperbarui"})
}

// Hapus film (DELETE)
func deleteMovie(w http.ResponseWriter, _ *http.Request, id int) {
	_, err := db.Exec("DELETE FROM movies WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Film berhasil dihapus"})
}
