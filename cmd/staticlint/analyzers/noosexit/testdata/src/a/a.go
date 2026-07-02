package main

import (
	"log"
	"os"
)

func main() {
	os.Exit(1) // want "не используйте os.Exit в функции main"
}

func helper() {
	os.Exit(1)         // want "не используйте os.Exit вне функции main"
	log.Fatal("error") // want "не используйте log.Fatal вне функции main"
}
