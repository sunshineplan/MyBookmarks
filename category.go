package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
)

type category struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func getCategoryID(category string, userID int, db *sql.DB) (int, error) {
	if category != "" {
		var categoryID int
		err := db.QueryRow("SELECT id FROM category WHERE category = ? AND user_id = ?", category, userID).Scan(&categoryID)
		switch {
		case len(category) > 15:
			return -1, nil
		case err != nil:
			if err == sql.ErrNoRows {
				res, err := db.Exec("INSERT INTO category (category, user_id) VALUES (?, ?)", category, userID)
				if err != nil {
					log.Println("Failed to add category:", err)
					return 0, err
				}
				lastInsertID, err := res.LastInsertId()
				if err != nil {
					log.Println("Failed to get last insert id:", err)
					return 0, err
				}
				return int(lastInsertID), nil
			}
			return 0, err
		default:
			return categoryID, nil
		}
	} else {
		return 0, nil
	}
}

func getCategory(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	rows, err := db.Query("SELECT id, category, count FROM categories WHERE user_id = ?", userID)
	if err != nil {
		log.Println("Failed to get categories:", err)
		c.String(500, "")
		return
	}
	defer rows.Close()
	categories := []category{}
	for rows.Next() {
		var category category
		if err := rows.Scan(&category.ID, &category.Name, &category.Count); err != nil {
			log.Println("Failed to scan category:", err)
			c.String(500, "")
			return
		}
		categories = append(categories, category)
	}

	var uncategorized int
	if err := db.QueryRow("SELECT count(bookmark) num FROM bookmark WHERE category_id = 0 AND user_id = ?",
		userID).Scan(&uncategorized); err != nil {
		log.Println("Failed to scan uncategorized bookmark count:", err)
		c.String(500, "")
		return
	}
	if uncategorized != 0 {
		categories = append(categories, category{ID: 0, Name: "Uncategorized", Count: uncategorized})
	}

	c.JSON(200, categories)
}

func addCategory(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	var category category
	if err := c.BindJSON(&category); err != nil {
		c.String(400, "")
		return
	}

	var message string
	switch {
	case category.Name == "":
		message = "Category name is empty."
	case len(category.Name) > 15:
		message = "Category name exceeded length limit."
	default:
		var exist string
		if err := db.QueryRow("SELECT id FROM category WHERE category = ? AND user_id = ?",
			category.Name, userID).Scan(&exist); err == nil {
			message = fmt.Sprintf("Category %s is already existed.", category.Name)
		} else {
			if err == sql.ErrNoRows {
				if _, err := db.Exec("INSERT INTO category (category, user_id) VALUES (?, ?)",
					category.Name, userID); err != nil {
					log.Println("Failed to add category:", err)
					c.String(500, "")
					return
				}
				c.JSON(200, gin.H{"status": 1})
				return
			}
			log.Println("Failed to scan category:", err)
			c.String(500, "")
			return
		}
	}
	c.JSON(200, gin.H{"status": 0, "message": message, "error": 1})
}

func editCategory(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Println("Failed to get id param:", err)
		c.String(400, "")
		return
	}

	var oldCategory string
	if err := db.QueryRow("SELECT category FROM category WHERE id = ? AND user_id = ?",
		id, userID).Scan(&oldCategory); err != nil {
		log.Println("Failed to scan category:", err)
		c.String(500, "")
		return
	}

	var category category
	if err := c.BindJSON(&category); err != nil {
		c.String(400, "")
		return
	}

	var message string
	var errorCode int
	switch {
	case category.Name == "":
		message = "New category name is empty."
		errorCode = 1
	case oldCategory == category.Name:
		message = "New category is same as old category."
	case len(category.Name) > 15:
		message = "Category name exceeded length limit."
		errorCode = 1
	default:
		var exist string
		if err := db.QueryRow("SELECT id FROM category WHERE category = ? AND user_id = ?",
			category.Name, userID).Scan(&exist); err == nil {
			message = fmt.Sprintf("Category %s is already existed.", category.Name)
			errorCode = 1
		} else {
			if _, err := db.Exec("UPDATE category SET category = ? WHERE id = ? AND user_id = ?",
				category.Name, id, userID); err != nil {
				log.Println("Failed to edit category:", err)
				c.String(500, "")
				return
			}
			c.JSON(200, gin.H{"status": 1})
			return
		}
	}
	c.JSON(200, gin.H{"status": 0, "message": message, "error": errorCode})
}

func deleteCategory(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()
	session := sessions.Default(c)
	userID := session.Get("user_id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Println("Failed to get id param:", err)
		c.String(400, "")
		return
	}

	if _, err := db.Exec("DELETE FROM category WHERE id = ? and user_id = ?",
		id, userID); err != nil {
		log.Println("Failed to delete category:", err)
		c.String(500, "")
		return
	}
	if _, err := db.Exec("UPDATE bookmark SET category_id = 0 WHERE category_id = ? and user_id = ?",
		id, userID); err != nil {
		log.Println("Failed to remove deleted category for bookmark:", err)
		c.String(500, "")
		return
	}
	c.JSON(200, gin.H{"status": 1})
}
