package main

import (
	"database/sql"
	"github.com/doug-martin/goqu/v9"
)

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (p *Product) getProduct(db *sql.DB) error {
	err := db.QueryRow("SELECT name, price FROM Products WHERE id=$1", p.ID).Scan(&p.Name, &p.Price)
	if err != nil {
		return err
	}
	return nil
}

func getProductsByIDList(db *sql.DB, ids []int) ([]Product, error) {
	if len(ids) > 0 {
		query, _, err := goqu.Dialect("postgres").
			From("products").
			Where(goqu.Ex{
				"id": ids,
			}).ToSQL()

		rows, err := db.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var products []Product
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
				return nil, err
			}
			products = append(products, p)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		return products, nil
	}
	return []Product{}, nil
}

func (p *Product) updateProduct(db *sql.DB) error {
	_, err := db.Exec("UPDATE Products SET name=$1, price=$2 WHERE id=$3", p.Name, p.Price, p.ID)
	return err
}

func (p *Product) deleteProduct(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM Products WHERE id=$1", p.ID)
	return err
}

func (p *Product) createProduct(db *sql.DB) error {
	err := db.QueryRow("INSERT INTO Products(name, price) VALUES($1, $2) RETURNING id", p.Name, p.Price).Scan(&p.ID)
	if err != nil {
		return err
	}
	return nil
}

func insertOrGetProducts(db *sql.DB, products []Product) ([]int, error) {
	var ids []int
	for _, p := range products {
		var id int
		err := db.QueryRow("SELECT id FROM Products WHERE id=$1 OR (name=$2 AND price=$3)", p.ID, p.Name, p.Price).Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			err = db.QueryRow("INSERT INTO Products(name, price) VALUES($1, $2) RETURNING id", p.Name, p.Price).Scan(&id)
			if err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func searchProducts(db *sql.DB, query string) ([]Product, error) {
	rows, err := db.Query("SELECT * FROM Products WHERE name LIKE '%' || $1 || '%'", query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

func getProducts(db *sql.DB, start, count int) ([]Product, error) {
	rows, err := db.Query("SELECT * FROM Products ORDER BY id LIMIT $1 OFFSET $2", count, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}
