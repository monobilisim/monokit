package healthdb

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	colReset = "\033[0m"
	colKey   = "\033[33m" // yellow
	colStr   = "\033[32m" // green
	colNum   = "\033[36m" // cyan
	colBool  = "\033[35m" // magenta
	colNull  = "\033[90m" // bright black
)

// NewCmd returns a cobra.Command group for inspecting the health DB
func NewCmd() *cobra.Command {
	// default from env (1/true => disable colors)
	defaultNoColors := false
	if v := strings.ToLower(os.Getenv("MONOKIT_NOCOLOR")); v == "1" || v == "true" {
		defaultNoColors = true
	}
	var noColors bool

	cmd := &cobra.Command{
		Use:   "db",
		Short: "Inspect monokit health SQLite database",
	}

	cmd.PersistentFlags().BoolVar(&noColors, "no-colors", defaultNoColors, "Disable colored JSON output")

	cmd.AddCommand(newPathCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd(&noColors))
	cmd.AddCommand(newDumpCmd(&noColors))
	return cmd
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the SQLite DB path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getDefaultDBPath())
		},
	}
}

func newListCmd() *cobra.Command {
	var module string
	c := &cobra.Command{
		Use:   "list",
		Short: "List keys (optionally scoped by --module)",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := Get()
			type Row struct{ Module, K string }
			var rows []Row
			q := db.Model(&KVEntry{}).Select("module, k").Order("module, k")
			if module != "" {
				q = q.Where("module = ?", module)
			}
			if err := q.Find(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				fmt.Printf("%s\t%s\n", r.Module, r.K)
			}
			return nil
		},
	}
	c.Flags().StringVar(&module, "module", "", "Filter by module")
	return c
}

func newGetCmd(noColors *bool) *cobra.Command {
	var module string
	c := &cobra.Command{
		Use:   "get <key>",
		Short: "Get value for a key (requires --module)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if module == "" {
				return fmt.Errorf("--module is required")
			}
			key := args[0]
			jsonStr, _, _, found, err := GetJSON(module, key)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("not found: %s/%s", module, key)
			}
			pretty, perr := prettyJSON(jsonStr)
			if perr != nil {
				// Fallback to raw
				fmt.Println(jsonStr)
				return nil
			}
			fmt.Print(colorizeJSON(pretty, *noColors))
			return nil
		},
	}
	c.Flags().StringVar(&module, "module", "", "Module name")
	return c
}

func newDumpCmd(noColors *bool) *cobra.Command {
	var module string
	c := &cobra.Command{
		Use:   "dump",
		Short: "Dump all entries as JSON (optionally filter by --module)",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := Get()
			var entries []KVEntry
			q := db.Order("module, k")
			if module != "" {
				q = q.Where("module = ?", module)
			}
			if err := q.Find(&entries).Error; err != nil {
				return err
			}
			b, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			fmt.Print(colorizeJSON(string(b), *noColors))
			return nil
		},
	}
	c.Flags().StringVar(&module, "module", "", "Filter by module")
	return c
}

// prettyJSON re-indents a raw JSON string
func prettyJSON(s string) (string, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// colorizeJSON adds ANSI colors to a pretty-printed JSON string
func colorizeJSON(in string, noColors bool) string {
	if noColors {
		return in
	}
	var out strings.Builder
	lines := strings.Split(in, "\n")
	for _, line := range lines {
		out.WriteString(colorizeLine(line))
		out.WriteByte('\n')
	}
	return out.String()
}

func colorizeLine(line string) string {
	// Write indentation
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	indent := line[:i]
	rest := line[i:]

	// If line doesn't start with a JSON key, return as-is
	if len(rest) == 0 || rest[0] != '"' {
		return line
	}

	// Parse key string ending, handling escapes
	keyStart := 0 // within rest
	j := 1
	esc := false
	for j < len(rest) {
		c := rest[j]
		if c == '\\' && !esc {
			esc = true
			j++
			continue
		}
		if c == '"' && !esc {
			break
		}
		esc = false
		j++
	}
	if j >= len(rest) {
		return line // malformed, give up
	}
	keyToken := rest[keyStart : j+1] // includes quotes

	// Expect ":"
	k := j + 1
	for k < len(rest) && (rest[k] == ' ' || rest[k] == '\t') {
		k++
	}
	if k >= len(rest) || rest[k] != ':' {
		return line
	}
	k++ // skip ':'
	for k < len(rest) && rest[k] == ' ' {
		k++
	}

	// Color key
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(colKey)
	b.WriteString(keyToken)
	b.WriteString(colReset)
	b.WriteString(": ")

	// Color value based on first char
	if k >= len(rest) {
		return b.String()
	}
	c := rest[k]
	switch {
	case c == '"': // string value
		valEnd := k + 1
		esc = false
		for valEnd < len(rest) {
			d := rest[valEnd]
			if d == '\\' && !esc {
				esc = true
				valEnd++
				continue
			}
			if d == '"' && !esc {
				break
			}
			esc = false
			valEnd++
		}
		if valEnd < len(rest) {
			valEnd++
		}
		b.WriteString(colStr)
		b.WriteString(rest[k:valEnd])
		b.WriteString(colReset)
		b.WriteString(rest[valEnd:])
	case isDigit(c) || c == '-': // number
		valEnd := k
		for valEnd < len(rest) && (isDigit(rest[valEnd]) || strings.ContainsRune("+-.eE", rune(rest[valEnd]))) {
			valEnd++
		}
		b.WriteString(colNum)
		b.WriteString(rest[k:valEnd])
		b.WriteString(colReset)
		b.WriteString(rest[valEnd:])
	case strings.HasPrefix(rest[k:], "true") || strings.HasPrefix(rest[k:], "false"):
		word := "true"
		if strings.HasPrefix(rest[k:], "false") {
			word = "false"
		}
		b.WriteString(colBool)
		b.WriteString(word)
		b.WriteString(colReset)
		b.WriteString(rest[k+len(word):])
	case strings.HasPrefix(rest[k:], "null"):
		b.WriteString(colNull)
		b.WriteString("null")
		b.WriteString(colReset)
		b.WriteString(rest[k+4:])
	default:
		b.WriteString(rest[k:]) // objects/arrays
	}
	return b.String()
}
