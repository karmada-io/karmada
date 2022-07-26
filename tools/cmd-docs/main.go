package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	// set HOME env var so that default values involve user's home directory do not depend on the running user.
	os.Setenv("HOME", "/home/user")
	os.Setenv("XDG_CONFIG_HOME", "/home/user/.config")

	err := doc.GenMarkdownTree(karmadactl.NewKarmadaCtlCommand("karmadactl", "karmadactl"), "./docs/userguide/commands")
	if err != nil {
		log.Fatal(err)
	}
}
