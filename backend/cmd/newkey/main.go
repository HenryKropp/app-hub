// Command newkey prints a fresh HUB_SECRET_KEY value.
package main

import (
	"fmt"

	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

func main() { fmt.Println(vault.NewKey()) }
