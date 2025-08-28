package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

var (
	// SERVER
	AppName       string
	Port          string
	PreFork       bool
	LoggerEnabled bool
	// ENV
	IsProduction  bool
	IsDevelopment bool
)

const (
	Development = "development"
	Production  = "production"
)

func init() {
	_ = godotenv.Load()
	LoadConfig()
}

func LoadConfig() {
	AppName = os.Getenv("APP_NAME")

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		Port = ":8080"
	} else {
		Port = ":" + strconv.Itoa(port)
	}

	// Booleans
	PreFork = os.Getenv("PREFORK") == "true"
	LoggerEnabled = os.Getenv("LOGGER_ENABLED") == "true"

	// ENV
	env := os.Getenv("ENV")
	IsDevelopment = env == "" || env == Development
	IsProduction = env == Production
	log.Println("Configuration loaded successfully")
}
