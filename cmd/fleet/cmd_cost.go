package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/cost"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/global"
)

type costRow struct {
	Agent  string         `json:"agent"`
	Driver string         `json:"driver"`
	Cost   cost.AgentCost `json:"cost"`
}

func dollars(ac cost.AgentCost) string {
	if !ac.Available {
		return "—"
	}
	return fmt.Sprintf("$%.2f", ac.TotalCostUSD)
}

func renderCostTable(repoName string, rows []costRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repo: %s\n", repoName)
	fmt.Fprintf(&b, "%-20s %-14s %-26s %s\n", "AGENT", "DRIVER", "MODEL(S)", "COST")
	var total float64
	for _, r := range rows {
		models := strings.Join(r.Cost.Models, ",")
		if models == "" {
			models = "—"
		}
		fmt.Fprintf(&b, "%-20s %-14s %-26s %s\n", r.Agent, r.Driver, models, dollars(r.Cost))
		if r.Cost.Available {
			total += r.Cost.TotalCostUSD
		}
	}
	fmt.Fprintf(&b, "%-20s %-14s %-26s $%.2f\n", "TOTAL", "", "", total)
	return b.String()
}

func renderCostJSON(rows []costRow) string {
	out, _ := json.MarshalIndent(rows, "", "  ")
	return string(out)
}

// costRowsForFleet builds a cost row per agent, using cached ccusage reports.
func costRowsForFleet(f *fleet.Fleet) []costRow {
	rows := make([]costRow, 0, len(f.Agents))
	for _, a := range f.Agents {
		row := costRow{Agent: a.Name, Driver: a.Driver}
		if source, ok := cost.DriverSource(a.Driver); ok {
			if report, err := cost.Report(source); err == nil {
				if ac, found := cost.MatchProjectKey(a.WorktreePath, report); found {
					row.Cost = ac
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func runCost(asJSON bool) error {
	if !cost.Available() {
		fmt.Fprintln(os.Stderr, "cost reporting requires ccusage — install it with: npm i -g ccusage")
		os.Exit(1)
	}
	f, err := fleet.Load(".")
	if err != nil {
		return err
	}
	rows := costRowsForFleet(f)
	if asJSON {
		fmt.Println(renderCostJSON(rows))
		return nil
	}
	fmt.Print(renderCostTable(f.ShortName, rows))
	return nil
}

func runCostAll(asJSON bool) error {
	if !cost.Available() {
		fmt.Fprintln(os.Stderr, "cost reporting requires ccusage — install it with: npm i -g ccusage")
		os.Exit(1)
	}
	repos, err := global.List()
	if err != nil {
		return err
	}
	var allRows []costRow
	for _, r := range repos {
		f, err := fleet.Load(r.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load fleet at %s: %v\n", r.Path, err)
			continue
		}
		rows := costRowsForFleet(f)
		if asJSON {
			allRows = append(allRows, rows...)
			continue
		}
		fmt.Print(renderCostTable(r.ShortName, rows))
		fmt.Println()
	}
	if asJSON {
		fmt.Println(renderCostJSON(allRows))
	}
	return nil
}
