// Command interfaces
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rjeczalik/interfaces"
	"golang.org/x/tools/go/packages"
)

var (
	optRpath    = flag.String("for", "", "Relative path to generate an interface for.")
	optTypename = flag.String("type", "", "Target struct name.")
	optAs       = flag.String("as", "", "Generated interface name.")
	optOutput   = flag.String("out", "-", "Output file.")
	optAll      = flag.Bool("all", false, "Include also unexported methods.")
)

var (
	errEmptyForOpt      = errors.New("empty -for option value")
	errEmptyTypeNameOpt = errors.New("empty -type option value")
	errEmptyAsOpt       = errors.New("empty -as option value")
	errEmptyOutputOpt   = errors.New("empty -out option value")
)

var tmpl = template.Must(template.New("").Parse(`// Code generated by interfacer; DO NOT EDIT

package {{.PackageName}}

import (
{{range .Deps}}	"{{.}}"
{{end}})

// {{.InterfaceName}} is an interface generated for {{.Type}}.
type {{.InterfaceName}} interface {
{{range .Interface}}	{{.}}
{{end}}}
`))

type vars struct {
	PackageName   string
	InterfaceName string
	Type          string
	Deps          []string
	Interface     interfaces.Interface
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageInfo(dir string) (string, error) {
	dir = filepath.ToSlash(dir)

	if strings.HasPrefix(dir, "./") || strings.HasPrefix(dir, "../") {
		cfg := new(packages.Config)
		cfg.Dir = dir

		pkgs, err := packages.Load(cfg, "./")
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}

		if len(pkgs) < 1 {
			log.Fatalf("No packages found in %s", dir)
		}

		return pkgs[0].String(), nil
	}

	return dir, nil
}

func parseQuery(forPackage, typename string) (*interfaces.Query, error) {
	query := forPackage + "." + typename

	q, err := interfaces.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return q, nil
}

func createInterface(q *interfaces.Query, all bool) (interfaces.Interface, error) {
	opts := new(interfaces.Options)
	opts.Query = q
	opts.Unexported = all

	itf, err := interfaces.NewWithOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return itf, nil
}

func generateCode(v *vars) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return out, nil
}

func writeOutput(output string, formatted []byte) error {
	outputWriter := os.Stdout

	if output != "-" {
		var (
			err             error
			filePermissions fs.FileMode
		)

		filePermissions = 0o644

		outputWriter, err = os.OpenFile(output, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, filePermissions)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		defer outputWriter.Close()
	}

	if _, err := outputWriter.Write(formatted); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func interfacer() error {
	forPackage, err := packageInfo(*optRpath)
	if err != nil {
		return err
	}

	parsedQuery, err := parseQuery(forPackage, *optTypename)
	if err != nil {
		return err
	}

	generatedInterface, err := createInterface(parsedQuery, *optAll)
	if err != nil {
		return err
	}

	variables := new(vars)
	variables.Type = fmt.Sprintf(`"%s"`, parsedQuery.Package+"."+parsedQuery.TypeName)
	variables.Deps = generatedInterface.Deps()
	variables.Interface = generatedInterface

	if i := strings.IndexRune(*optAs, '.'); i != -1 {
		variables.PackageName = (*optAs)[:i]
		variables.InterfaceName = (*optAs)[i+1:]
	} else {
		variables.InterfaceName = *optAs
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return fmt.Errorf("%w", err)
	}

	formatted, err := generateCode(variables)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return writeOutput(*optOutput, formatted)
}

func run() error {
	flag.Parse()

	if *optRpath == "" {
		return fmt.Errorf("%w", errEmptyForOpt)
	}

	if *optTypename == "" {
		return fmt.Errorf("%w", errEmptyTypeNameOpt)
	}

	if *optAs == "" {
		return fmt.Errorf("%w", errEmptyAsOpt)
	}

	if *optOutput == "" {
		return fmt.Errorf("%w", errEmptyOutputOpt)
	}

	if err := interfacer(); err != nil {
		return err
	}

	return nil
}
