package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"io"
	"log"
	"net/http"
	"strconv"
)

type CheckOut struct {
	products []Product
	subtotal float64
}

type App struct {
	Router *mux.Router
	DB     *sql.DB
}

func (a *App) Initialize(user, password, dbname string) {
	fmt.Printf(user, password, dbname)
	connectionString :=
		fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", user, password, dbname)

	var err error
	a.DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}

	err = a.DB.Ping()
	if err != nil {
		log.Fatal(err)
	}

	a.Router = mux.NewRouter()
	a.initializeRoutes()
}

func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(addr, a.Router))
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/Products", a.getProducts).Methods("GET")
	a.Router.HandleFunc("/product", a.createProduct).Methods("POST")
	a.Router.HandleFunc("/product/{id:[0-9]+}", a.getProduct).Methods("GET")
	a.Router.HandleFunc("/product/{id:[0-9]+}", a.updateProduct).Methods("PUT")
	a.Router.HandleFunc("/product/{id:[0-9]+}", a.deleteProduct).Methods("DELETE")
	a.Router.HandleFunc("/product/search", a.searchProducts).Methods("GET")

	a.Router.HandleFunc("/cart", a.addProductsToCart).Methods("PUT")
	a.Router.HandleFunc("/cart/{id}", a.getCart).Methods("GET")
	a.Router.HandleFunc("/cart/{id}/products", a.removeProductsFromCart).Methods("DELETE")
	a.Router.HandleFunc("/cart/{id}", a.clearCart).Methods("DELETE")
	a.Router.HandleFunc("/cart/{id}/checkout", a.checkout).Methods("GET")
}

func (a *App) getProducts(w http.ResponseWriter, r *http.Request) {
	countStr := r.FormValue("count")
	startStr := r.FormValue("start")
	var count, start int
	var err error

	if countStr != "" {
		count, err = strconv.Atoi(countStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid count parameter")
			return
		}
	} else {
		count = 10
	}

	if startStr != "" {
		start, err = strconv.Atoi(startStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid start parameter")
			return
		}
	} else {
		start = 0
	}

	products, err := getProducts(a.DB, start, count)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, products)
}

func (a *App) createProduct(w http.ResponseWriter, r *http.Request) {
	var p Product
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&p); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Print("Error closing")
		}
	}(r.Body)

	if err := p.createProduct(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, p)
}

func (a *App) getProduct(w http.ResponseWriter, r *http.Request) {
	param := mux.Vars(r)
	id, err := strconv.Atoi(param["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Product ID")
		return
	}

	p := Product{ID: id}
	if err := p.getProduct(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "Product not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondWithJSON(w, http.StatusOK, p)
}

func (a *App) updateProduct(w http.ResponseWriter, r *http.Request) {
	param := mux.Vars(r)
	id, err := strconv.Atoi(param["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Product ID")
		return
	}

	var p Product
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&p); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Print("Error closing db")
		}
	}(r.Body)
	p.ID = id

	if err := p.updateProduct(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, p)
}

func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	param := mux.Vars(r)
	id, err := strconv.Atoi(param["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Product ID")
		return
	}

	p := Product{ID: id}
	if err := p.deleteProduct(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) addProductsToCart(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") == "" {
		respondWithError(w, http.StatusNotFound, "No Content type header added!")
	}
	var reqBody struct {
		CartID   int       `json:"cart_id"`
		Products []Product `json:"products"`
	}

	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if reqBody.CartID == 0 {
		reqBody.CartID = -1
	}
	cart := &Cart{ID: reqBody.CartID}
	err = cart.getOrCreate(a.DB)
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusNotFound, "Cart not found")
		return
	}

	if err := cart.addProducts(a.DB, reqBody.Products); err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Product was not found"))
		return
	}

	respondWithJSON(w, http.StatusOK, cart)
}

func (a *App) removeProductsFromCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Cart ID")
		return
	}
	var reqBody struct {
		ProductIds []int `json:"product_ids"`
	}

	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	cart := &Cart{ID: cartID}
	err = cart.getCart(a.DB)
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusNotFound, "Cart not found")
		return
	}

	if err := cart.removeProducts(a.DB, reqBody.ProductIds); err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Product was not found"))
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Products removed from Cart successfully"})
}

func (a *App) getCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID, err := strconv.Atoi(vars["id"])
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, "Invalid Cart ID")
		return
	}

	cart := Cart{ID: cartID}
	err = cart.getCart(a.DB)
	if err != nil {
		fmt.Print(err)
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "Cart not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondWithJSON(w, http.StatusOK, cart)
}

func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID, err := strconv.Atoi(vars["id"])
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, "Invalid Cart ID")
		return
	}

	cart := Cart{ID: cartID}
	products, subtotal, err := cart.checkOut(a.DB)
	if err != nil {
		fmt.Print(err)
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "Cart not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondWithJSON(w, http.StatusOK, CheckOut{products: products, subtotal: subtotal})
}

func (a *App) clearCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID, err := strconv.Atoi(vars["id"])
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusBadRequest, "Invalid Cart ID")
		return
	}

	cart := Cart{ID: cartID}
	if err := cart.clear(a.DB); err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) searchProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "Missing search query")
		return
	}

	products, err := searchProducts(a.DB, query)
	if err != nil {
		fmt.Print(err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, products)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(response)
	if err != nil {
		fmt.Print("Error writing to db")
		return
	}
}
