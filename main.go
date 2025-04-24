package main

import (
	"net/http"
)

func main() {
	server := http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	http.HandleFunc("/recipes", handleRecipes)
	http.HandleFunc("/recipes/", handleRecipesWithId)

	server.ListenAndServe()
}

type Recipe struct {
	Title       string
	MakingTime  string
	Serves      string
	Ingredients string
	Cost        string
}

func handleRecipes(w http.ResponseWriter, r *http.Request) {
}

func handleRecipesWithId(w http.ResponseWriter, r *http.Request) {
}
