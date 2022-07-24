package init

import (
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
)

// CommandAddonsListOption options for addons list.
type CommandAddonsListOption struct {
	GlobalCommandOptions
}

// Complete the conditions required to be able to run list.
func (o *CommandAddonsListOption) Complete() error {
	return o.GlobalCommandOptions.Complete()
}

// Run start list Karmada addons
func (o *CommandAddonsListOption) Run() error {
	addonNames := make([]string, 0, len(Addons))
	for addonName := range Addons {
		addonNames = append(addonNames, addonName)
	}
	sort.Strings(addonNames)

	// Init tableWriter
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoFormatHeaders(true)
	table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	table.SetCenterSeparator("|")

	// Create table header
	tHeader := []string{"Addon Name", "Status"}
	table.SetHeader(tHeader)

	// Create table data
	var tData [][]string
	var temp []string
	for _, addonName := range addonNames {
		tStatus, err := Addons[addonName].Status(o)
		if err != nil {
			return err
		}
		temp = []string{addonName, tStatus}
		tData = append(tData, temp)
	}

	table.AppendBulk(tData)

	table.Render()

	return nil
}
