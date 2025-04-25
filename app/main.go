package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	const dbDriver = "mysql"
	const dbServer = "db:3306"
	const dbUser = "user"
	const dbPassword = "password"
	const dbName = "recipes"
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPassword, dbServer, dbName)
	db, err := sql.Open(dbDriver, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	server := http.Server{
		Addr:    ":8080",
		Handler: nil,
	}
	appHandler := &AppHandler{DB: db}
	http.HandleFunc("/recipes", appHandler.handleRecipes)
	http.HandleFunc("/recipes/", appHandler.handleRecipesWithId)

	log.Println("Server started on :8080")
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

type AppHandler struct {
	DB *sql.DB
}

func (h *AppHandler) handleRecipes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := h.DB.Query("SELECT id, title, making_time, serves, ingredients, cost FROM recipes")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var recipes []RecipeRowWOTimestamp
		for rows.Next() {
			var recipe RecipeRowWOTimestamp
			_ = rows.Scan(&recipe.ID, &recipe.Title, &recipe.MakingTime, &recipe.Serves, &recipe.Ingredients, &recipe.Cost)
			recipes = append(recipes, recipe)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecipeListGetResponse{Recipes: recipes})
	case http.MethodPost:
		parseError := func() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RecipePostFailureResponse{
				Message:  "Recipe creation failed!",
				Required: "title, making_time, serves, ingredients, cost",
			})
		}
		var recipe RecipePostRequest
		err := json.NewDecoder(r.Body).Decode(&recipe)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !validatePostFields(recipe) {
			parseError()
			return
		}
		result, err := h.DB.Exec("INSERT INTO recipes (title, making_time, serves, ingredients, cost) VALUES (?, ?, ?, ?, ?)",
			recipe.Title, recipe.MakingTime, recipe.Serves, recipe.Ingredients, recipe.Cost)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var insertedRow RecipeRow
		err = h.DB.QueryRow("SELECT id, title, making_time, serves, ingredients, cost, created_at, updated_at FROM recipes WHERE id = ?", id).Scan(
			&insertedRow.ID,
			&insertedRow.Title,
			&insertedRow.MakingTime,
			&insertedRow.Serves,
			&insertedRow.Ingredients,
			&insertedRow.Cost,
			&insertedRow.CreatedAt,
			&insertedRow.UpdatedAt,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecipePostOkResponse{
			Message: "Recipe successfully created!",
			Recipe:  []RecipeRow{insertedRow},
		})
	}
}

func (h *AppHandler) handleRecipesWithId(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Path[len("/recipes/"):])
	switch r.Method {
	case http.MethodGet:
		var recipe RecipeRowWOTimestamp
		err := h.DB.QueryRow("SELECT id, title, making_time, serves, ingredients, cost FROM recipes WHERE id = ?", id).Scan(
			&recipe.ID,
			&recipe.Title,
			&recipe.MakingTime,
			&recipe.Serves,
			&recipe.Ingredients,
			&recipe.Cost,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Recipe not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecipeGetResponse{
			Message: "Recipe details by id",
			Recipe:  []RecipeRowWOTimestamp{recipe},
		})
	case http.MethodPatch:
		var recipe RecipePatchRequest
		err := json.NewDecoder(r.Body).Decode(&recipe)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !validatePatchFields(recipe) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RecipePostFailureResponse{
				Message:  "Recipe update failed!",
				Required: "title, making_time, serves, ingredients, cost",
			})
			return
		}
		_, err = h.DB.Exec("UPDATE recipes SET title = ?, making_time = ?, serves = ?, ingredients = ?, cost = ? WHERE id = ?",
			recipe.Title, recipe.MakingTime, recipe.Serves, recipe.Ingredients, recipe.Cost, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecipePatchOkResponse{
			Message: "Recipe successfully updated!",
			Recipe:  []RecipePatchRequest{recipe},
		})
	case http.MethodDelete:
		var count int
		err := h.DB.QueryRow("SELECT COUNT(*) FROM recipes WHERE id = ?", id).Scan(&id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if count == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RecipeDeleteFailureResponse{
				Message: "No Recipe found",
			})
			return
		}
		_, err = h.DB.Exec("DELETE FROM recipes WHERE id = ?", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecipeDeleteOkResponse{
			Message: "Recipe successfully removed!",
		})
	}
}

func validatePostFields(recipe RecipePostRequest) bool {
	if recipe.Title == "" || recipe.MakingTime == "" || recipe.Serves == "" || recipe.Ingredients == "" || recipe.Cost <= 0 {
		return false
	}
	return true
}

func validatePatchFields(recipe RecipePatchRequest) bool {
	if recipe.Title == "" || recipe.MakingTime == "" || recipe.Serves == "" || recipe.Ingredients == "" || recipe.Cost <= 0 {
		return false
	}
	return true
}

type RecipeRow struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	MakingTime  string `json:"making_time"`
	Serves      string `json:"serves"`
	Ingredients string `json:"ingredients"`
	Cost        int    `json:"cost"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type RecipeRowWOTimestamp struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	MakingTime  string `json:"making_time"`
	Serves      string `json:"serves"`
	Ingredients string `json:"ingredients"`
	Cost        int    `json:"cost"`
}

type RecipePostRequest struct {
	Title       string `json:"title"`
	MakingTime  string `json:"making_time"`
	Serves      string `json:"serves"`
	Ingredients string `json:"ingredients"`
	Cost        int    `json:"cost"`
}

type RecipePostOkResponse struct {
	Message string      `json:"message"`
	Recipe  []RecipeRow `json:"recipe"`
}

type RecipePostFailureResponse struct {
	Message  string `json:"message"`
	Required string `json:"required"`
}

type RecipeListGetResponse struct {
	Recipes []RecipeRowWOTimestamp `json:"recipes"`
}

type RecipeGetResponse struct {
	Message string                 `json:"message"`
	Recipe  []RecipeRowWOTimestamp `json:"recipe"`
}

type RecipePatchRequest struct {
	Title       string `json:"title"`
	MakingTime  string `json:"making_time"`
	Serves      string `json:"serves"`
	Ingredients string `json:"ingredients"`
	Cost        int    `json:"cost"`
}

type RecipePatchOkResponse struct {
	Message string               `json:"message"`
	Recipe  []RecipePatchRequest `json:"recipe"`
}

type RecipeDeleteOkResponse struct {
	Message string `json:"message"`
}

type RecipeDeleteFailureResponse struct {
	Message string `json:"message"`
}
