package main

import (
    "log"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
)

func main() {

    if err := godotenv.Load("../.env"); err != nil {
        log.Println("No .env file found, using environment variables")
    }

    r := gin.Default()

    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok", "service": "lembas-links"})
    })

    log.Println("Lembas Links API running on :8080")
    r.Run(":8080")
}
