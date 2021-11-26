package main

import "github.com/karmada-io/karmada/cmd/backend/app"

func main() {
	router := app.InitRouter()
	_ = router.Run(":8080")
}
