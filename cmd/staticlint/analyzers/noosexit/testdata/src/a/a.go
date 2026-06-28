package main

import "os"

func main() {
	os.Exit(1) // want "не используйте os.Exit в функции main"
}
