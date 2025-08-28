package common

import (
	"fmt"
	"os/exec"
	"strings"
)

func PostgresCheck() {
	// Check if PostgreSQL is installed by checking the existence of command "psql"
	_, err := exec.LookPath("psql")
	if err != nil {
		addToNotInstalled("PostgreSQL")
		return
	}

	// Get the version of PostgreSQL
	out, err := exec.Command("psql", "--version").Output()
	if err != nil {
		addToVersionErrors(fmt.Errorf("Error getting PostgreSQL version"))
		return
	}

	// Parse the version
	// Eg. output
	// psql (PostgreSQL) 13.3 (Ubuntu 13.3-1.pgdg20.04+1)
	version := strings.Split(string(out), " ")[2]

	oldVersion := GatherVersion("postgres")

	if oldVersion != "" && oldVersion != version {
		addToUpdated(AppVersion{Name: "PostgreSQL", OldVersion: oldVersion, NewVersion: version})
		CreateNews("PostgreSQL", oldVersion, version, false)
	} else {
		addToNotUpdated(AppVersion{Name: "PostgreSQL", OldVersion: version})
	}

	StoreVersion("postgres", version)
}
