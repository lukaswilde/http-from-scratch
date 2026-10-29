package main

import (
	"encoding/json/v2"
	"fmt"
	"httpServer/internal/http1"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/joho/godotenv"
)

type User struct {
	UserID int
	Name   string
}

func main() {

	var users []User

	users = append(users, User{1, "Frank"})
	users = append(users, User{3, "Hank"})

	http1.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			usersJSON, err := json.Marshal(users)
			log.Println(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			user := User{}
			err = json.Unmarshal(body, &user)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			users = append(users, user)
			w.Header().Set("Location", "/users/"+strconv.Itoa(user.UserID))
			w.WriteHeader(http.StatusCreated)
		}
	})
	http1.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			id, err := strconv.Atoi(r.PathValue("id"))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}

			idx := slices.IndexFunc(users, func(u User) bool {
				return u.UserID == id
			})

			if idx == -1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			user := users[idx]
			usersJSON, err := json.Marshal(user)
			log.Println(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(usersJSON)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	host := os.Getenv("HOST")
	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("Starting server at %s", addr)
	if err := http1.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
