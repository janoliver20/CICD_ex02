package CICD_ex02

import (
	"database/sql"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"time"
)

type Cart struct {
	ID                    int       `json:"id"`
	CreatedTimestamp      time.Time `json:"created_timestamp"`
	ModificationTimestamp time.Time `json:"modification_timestamp"`
	CheckedOut            bool      `json:"checked_out"`
	Products              []Product `json:"Products"`
}

func (c *Cart) addProducts(db *sql.DB, products []Product) error {
	if !c.CheckedOut {
		if len(products) > 0 {
			ids, err := insertOrGetProducts(db, products)
			if err != nil {
				fmt.Print(err)
				return err
			}
			err = c.linkCartToProducts(db, ids)
			if err != nil {
				return err
			}
			if err = c.updateCart(db); err != nil {
				return err
			}
			return c.getCart(db)
		}
		return nil
	}
	return fmt.Errorf("Cart already checked out!")
}

func (c *Cart) linkCartToProducts(db *sql.DB, productIds []int) error {
	if len(productIds) > 0 {

		var rows []goqu.Record
		for _, id := range productIds {
			rows = append(rows, goqu.Record{"cart_id": c.ID, "product_id": id})
		}
		query, _, err := goqu.Dialect("postgres").
			Insert("cart_products").
			OnConflict(goqu.DoNothing()).
			Rows(rows).
			ToSQL()
		if err != nil {
			return err
		}
		_, err = db.Exec(query)
		return err
	}
	return nil
}

func (c *Cart) removeProducts(db *sql.DB, productIds []int) error {
	if !c.CheckedOut {
		query, _, err := goqu.Dialect("postgres").Delete("cart_products").Where(goqu.Ex{
			"product_id": productIds,
		}).ToSQL()
		if err != nil {
			return err
		}

		if _, err = db.Exec(query); err != nil {
			return err
		}
		return c.getCart(db)
	}
	return fmt.Errorf("Cart already checked out!")
}

func (c *Cart) getOrCreate(db *sql.DB) error {
	if c.ID == -1 {
		return c.createCart(db)
	}
	err := c.getCart(db)
	if err != nil {
		err = c.createCart(db)
	}
	return err
}

func (c *Cart) getCart(db *sql.DB) error {
	err := db.QueryRow("SELECT id, created_timestamp, checked_out, modification_timestamp FROM carts WHERE id=$1", c.ID).
		Scan(&c.ID, &c.CreatedTimestamp, &c.CheckedOut, &c.ModificationTimestamp)
	if err != nil {
		return err
	}
	rows, err := db.Query("SELECT product_id FROM cart_products WHERE cart_id=$1", c.ID)

	var idSet []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		idSet = append(idSet, id)
	}

	products, err := getProductsByIDList(db, idSet)
	if err != nil {
		return err
	}
	c.Products = products
	return nil
}

func (c *Cart) createCart(db *sql.DB) error {
	if !c.CheckedOut {
		err := db.QueryRow("INSERT INTO carts DEFAULT VALUES RETURNING id").Scan(&c.ID)
		if err != nil {
			return err
		}
		if len(c.Products) > 0 {
			var vals []interface{}
			for _, product := range c.Products {
				vals = append(vals, goqu.Vals{c.ID, product.ID})
			}

			query, _, err := goqu.Dialect("postgres").
				Insert("cart_products").
				Cols("cart_id", "product_id").
				Vals(vals).ToSQL()

			if err != nil {
				fmt.Print(err)
				return err
			}

			return db.QueryRow(query).Err()
		}
		return nil
	}
	return fmt.Errorf("Cart already checked out!")
}

func (c *Cart) updateCart(db *sql.DB) error {
	err := db.QueryRow("SELECT id FROM carts WHERE id=$1", c.ID).Scan(&c.ID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("Cart with ID %d does not exist", c.ID)
	} else if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE carts SET checked_out=$1, modification_timestamp=$2 WHERE id=$3", c.CheckedOut, time.Now(), c.ID)
	if err != nil {
		return err
	}
	var productIds []int
	for _, product := range c.Products {
		productIds = append(productIds, product.ID)
	}
	return c.linkCartToProducts(db, productIds)
}

func (c *Cart) clear(db *sql.DB) error {
	if !c.CheckedOut {
		query, _, err := goqu.Dialect("postgres").
			Delete("cart_products").Where(goqu.Ex{
			"cart_id": c.ID,
		}).ToSQL()
		if err != nil {
			return err
		}
		if _, err = db.Exec(query); err != nil {
			return err
		}
		return c.getCart(db)
	}
	return fmt.Errorf("Cart already checked out!")
}

func (c *Cart) checkOut(db *sql.DB) ([]Product, float64, error) {
	if !c.CheckedOut {
		if err := c.getCart(db); err != nil {
			return []Product{}, 0, err
		}
		products := c.Products
		var subtotal float64 = 0
		for _, product := range products {
			subtotal += product.Price
		}
		c.CheckedOut = true
		if err := c.updateCart(db); err != nil {
			return []Product{}, 0, err
		}
		return products, subtotal, nil
	}
	return []Product{}, 0, fmt.Errorf("Cart already checked out!")
}

func subtotalOfCart(db *sql.DB, id int) (float64, error) {
	var cart Cart
	if err := db.QueryRow("SELECT * FROM carts WHERE id=$1", id).Scan(&cart); err != nil {
		return 0, err
	}
	var subtotal float64 = 0
	for _, product := range cart.Products {
		subtotal += product.Price
	}
	return subtotal, nil
}

func containsProduct(products []Product, id int) bool {
	for _, v := range products {
		if v.ID == id {
			return true
		}
	}
	return false
}
