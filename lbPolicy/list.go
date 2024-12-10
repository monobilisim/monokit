package lbPolicy

import (
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "strings"
    "github.com/olekukonko/tablewriter"
)

func InitList() []string {
	// Print the fixed headers
    res := []string{"SERVERS"}

	// Iterate over the input arguments
	for _, arg := range Config.Caddy.Api_Urls {
		// Extract the part after the semicolon (;)
		parts := strings.Split(arg, ";")
		if len(parts) > 1 {
			line := parts[1]
            res = append(res, line)
		}
	}

    return res
}

func ShowListMulti(args []string) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader(InitList())

    for _, arg := range args {
        table.Append(ShowList(arg))
    }

    table.Render()
}

func ShowList(arg string) []string {
	// Define the base path
	basePath := "/tmp/glb/" + arg

	// Check if the directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		fmt.Printf("warn: glb directory for %s doesn't exist, please run monokit lbPolicy switch first\n", arg)
		return []string{}
	}

	// Iterate through the files in the directory
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		fmt.Printf("warn: failed to read directory %s: %v\n", basePath, err)
		return []string{}
	}

    res := []string{arg}

	for _, file := range files {
		// Build the path to the lb_policy file
		lbPolicyPath := filepath.Join(basePath, file.Name(), "lb_policy")

		// Check if lb_policy is a file
		if stat, err := os.Stat(lbPolicyPath); err == nil && !stat.IsDir() {
			// Read the content of the lb_policy file
			content, err := ioutil.ReadFile(lbPolicyPath)
			if err != nil {
				fmt.Printf(" warn: failed to read %s: %v |", lbPolicyPath, err)
				continue
			}

			// Print the content of lb_policy
			//fmt.Printf(" %s |", string(content))
            res = append(res, string(content))
		}
	}

    return res
}
