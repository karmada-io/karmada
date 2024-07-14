/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
