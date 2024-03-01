package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/rs/cors"
)

var ctx = context.Background()
var client *redis.Client

var CHARACTERS = []string{
	"cat",
	"defuse",
	"shuffle",
	"exploding",
}

type PlayerData struct {
	ID      string `json:"id"`
	Player  string `json:"player"`
	Wins    int    `json:"wins"`
	Losses  int    `json:"losses"`
	Total   int    `json:"total"`
	Created string `json:"created"`
}

func generateRandomCards() []string {
	rand.Seed(time.Now().UnixNano())
	randomDeck := make([]string, 0)

	for i := 0; i < 5; i++ {
		index := rand.Intn(len(CHARACTERS))
		randomDeck = append(randomDeck, CHARACTERS[index])
	}

	return randomDeck
}

func main() {
	// Initialize Redis client
	client = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Redis server address
		DB:   0,
	})
	// Create a new Gin router
	router := gin.Default()

	// Enable CORS using the rs/cors package
	router.Use(corsMiddleware())

	// Define a handler for the root endpoint
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello, this is a Gin backend server with CORS support!")
	})

	// Define a handler for storing username in Redis
	router.POST("/store-username", storeUsernameHandler)

	// Define a handler for retrieving all stored usernames
	router.GET("/get-all-usernames", getAllUsernamesHandler)

	//random cards deck
	router.GET("/get-random-cards", func(c *gin.Context) {
		randomCards := generateRandomCards()
		c.JSON(http.StatusOK, gin.H{"cards": randomCards})
	})

	// Start the server on port 8080
	fmt.Println("Server is listening on port 8080...")
	err := router.Run(":8080")
	if err != nil {
		fmt.Println("Error:", err)
	}
}

// Function to set up CORS middleware using github.com/rs/cors
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a new CORS handler with default options
		corsHandler := cors.Default()

		// Allow all origins, headers, and methods
		corsHandler.HandlerFunc(c.Writer, c.Request)

		// Continue processing the request
		c.Next()
	}
}

// Handler for storing username in Redis
func storeUsernameHandler(c *gin.Context) {
	// Retrieve username from the request body
	var requestBody struct {
		Player string `json:"player" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a unique ID using the current timestamp
	id := strconv.FormatInt(time.Now().UnixNano(), 10)

	// Store username in Redis hash with associated ID, win counter, loss counter, and timestamp
	err := client.HMSet(ctx, "player:"+id, map[string]interface{}{
		"id":      id,
		"player":  requestBody.Player,
		"wins":    0,
		"losses":  0,
		"total":   0,
		"created": time.Now().Format(time.RFC3339),
	}).Err()

	if err != nil {
		fmt.Printf("Error storing player in Redis: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store player in Redis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "player stored successfully"})
}

// Handler for retrieving all stored usernames with stats
func getAllUsernamesHandler(c *gin.Context) {
	// Retrieve all usernames from Redis hashes
	userKeys, err := client.Keys(ctx, "player:*").Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve players from Redis"})
		return
	}

	// Get the details for each username
	var userStats []PlayerData
	for _, key := range userKeys {
		// Check if the key is a hash
		keyType, err := client.Type(ctx, key).Result()
		if err != nil {
			fmt.Printf("Error checking key type for key %s: %v\n", key, err)
			continue
		}

		if keyType != "hash" {
			fmt.Printf("Skipping non-hash key: %s\n", key)
			continue
		}

		userDetails, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			fmt.Printf("Error retrieving details for key %s: %v\n", key, err)
			continue
		}

		// Convert map[string]string to PlayerData struct
		var userData PlayerData
		userData.ID = userDetails["id"]
		userData.Player = userDetails["player"]
		userData.Wins, _ = strconv.Atoi(userDetails["wins"])
		userData.Losses, _ = strconv.Atoi(userDetails["losses"])
		userData.Total = userData.Wins + userData.Losses
		userData.Created = userDetails["created"]

		userStats = append(userStats, userData)
	}

	fmt.Printf("Retrieved PlayerStats: %+v\n", userStats)

	c.JSON(http.StatusOK, gin.H{"players": userStats})
}
