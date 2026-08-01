package main

import (
	"log"

	"github.com/GoScouter/sdk"
)

func main() {
	if err := sdk.Serve(&SgsModule{}); err != nil {
		log.Fatal(err)
	}
}
