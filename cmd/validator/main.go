package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/chop-dbhi/data-models-service/client"
	"github.com/chop-dbhi/data-models-validator"
	"github.com/olekukonko/tablewriter"
)

const DataModelsService = "http://data-models.origins.link"

var usage = `Data Models Validator - {{.Version}}

Usage:

  data-models-validator [-model <model>]
                        [-version <version>]
                        [-delim <delimiter>]
                        [-compr <compression>]
                        [-service <service>]
                        ( <file>[:<table>]... | [:<table>] )

The Data Models Validator reads a file containing data and checks it against
the data model's schema. Input files or stream are delimited files (such as CSV)
and optionally compressed using gzip or bzip2.

One or more existing input files can be explicitly passed otherwise STDIN
will be read. Each input can be optionally annotated with an explicit table name
to be validated against. If not specified, the file name will be used to
determine which table the file corresponds to.

The validator returns an exit status of 0 if no errors are found and nonzero
otherwise.

Source: https://github.com/chop-dbhi/data-models-validator

Examples:

  # Validate person.csv file against the OMOP v5 data model.
  data-models-validator -model omop -version 5.0.0 person.csv

  # Validate foo.csv against the person table in the OMOP v5 data model.
  data-models-validator -model omop -version 5.0.0 foo.csv:person

  # Validate the STDIN stream denoting it is tab-delimited and gzipped.
  data-models-validator -model omop -version 5.0.0 -delim $'\t' -compr gzip
`

func init() {
	var buf bytes.Buffer

	cxt := map[string]interface{}{
		"Version": validator.Version,
	}

	template.Must(template.New("usage").Parse(usage)).Execute(&buf, cxt)

	usage = buf.String()

	flag.Usage = func() {
		fmt.Println(usage)
	}
}

const sampleSize = 5

func main() {
	var (
		service   string
		modelName string
		version   string
		delim     string
		compr     string
	)

	flag.StringVar(&modelName, "model", "", "The model to validate against. Required.")
	flag.StringVar(&version, "version", "", "The specific version of the model to validate against. Defaults to the latest version of the model.")
	flag.StringVar(&service, "service", DataModelsService, "The data models service to use for fetching schema information.")

	flag.StringVar(&delim, "delim", ",", "The delimiter used in the input files or stream.")
	flag.StringVar(&compr, "compr", "", "The compression method used on the input files or stream. If ommitted the file extension will be used to infer the compression method: .gz, .gzip, .bzip2, .bz2.")

	flag.Parse()

	// Check required options.
	if modelName == "" {
		fmt.Println("A model must be specified.")
		os.Exit(1)
	}

	inputs := flag.Args()

	if len(inputs) == 0 {
		fmt.Println("At least one input must be specified.")
		os.Exit(1)
	}

	// Initialize data models client for service.
	c, err := client.New(service)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = c.Ping(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	revisions, err := c.ModelRevisions(modelName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var model *client.Model

	// Get the latest version.
	if version == "" {
		model = revisions.Latest()
	} else {

		var (
			versions []string
			_model   *client.Model
		)

		for _, _model = range revisions.List() {
			if _model.Version == version {
				model = _model
				break
			}

			versions = append(versions, _model.Version)
		}

		if model == nil {
			fmt.Printf("Invalid version for '%s'. Choose from: %s\n", modelName, strings.Join(versions, ", "))
			os.Exit(1)
		}
	}

	fmt.Printf("Validating against model '%s/%s'\n", model.Name, model.Version)

	var (
		hasErrors bool
		tableName string
		table     *client.Table
	)

	for _, name := range inputs {
		// The file name may have a suffix containing the table name, name[:table].
		// The fallback is to use the file name without the extension.
		toks := strings.SplitN(name, ":", 2)

		if len(toks) == 2 {
			name = toks[0]
			tableName = toks[1]
		} else {
			name = toks[0]

			toks = strings.SplitN(filepath.Base(name), ".", 2)
			tableName = toks[0]
		}

		if table = model.Tables.Get(tableName); table == nil {
			fmt.Printf("* Unknown table '%s'.\nChoices are: %s\n", tableName, strings.Join(model.Tables.Names(), ", "))
			continue
		}

		fmt.Printf("* Evaluating '%s' table in '%s'...\n", tableName, name)

		// Open the reader.
		reader, err := validator.Open(name, compr)

		if err != nil {
			fmt.Printf("* Could not open file: %s\n", err)
			continue
		}

		v := validator.New(reader, table)

		if err = v.Init(); err != nil {
			fmt.Printf("* Problem reading CSV header: %s\n", err)
			reader.Close()
			continue
		}

		if err = v.Run(); err != nil {
			fmt.Printf("* Problem reading CSV data: %s\n", err)
		}

		reader.Close()

		// Build the result.
		result := v.Result()

		lerrs := result.LineErrors()

		if len(lerrs) > 0 {
			hasErrors = true

			fmt.Println("* Row-level issues were found.")

			// Row level issues.
			tw := tablewriter.NewWriter(os.Stdout)

			tw.SetHeader([]string{
				"code",
				"error",
				"occurrences",
				"lines",
				"example",
			})

			var example string

			for err, verrs := range result.LineErrors() {
				ve := verrs[0]

				if ve.Context != nil {
					example = fmt.Sprintf("line %d: `%v` %v", ve.Line, ve.Value, ve.Context)
				} else {
					example = fmt.Sprintf("line %d: `%v`", ve.Line, ve.Value)
				}

				tw.Append([]string{
					fmt.Sprint(err.Code),
					err.Description,
					fmt.Sprint(len(verrs)),
					strings.Join(errLineSteps(verrs), ", "),
					example,
				})
			}

			tw.Render()
		}

		// Field level issues.
		tw := tablewriter.NewWriter(os.Stdout)

		tw.SetHeader([]string{
			"field",
			"code",
			"error",
			"occurrences",
			"lines",
			"samples",
		})

		var nerrs int

		// Output the error occurrence per field.
		for _, f := range v.Header {
			errmap := result.FieldErrors(f)

			if len(errmap) == 0 {
				continue
			}

			nerrs += len(errmap)

			var sample []*validator.ValidationError

			for err, verrs := range errmap {
				num := len(verrs)

				if num >= sampleSize {
					sample = make([]*validator.ValidationError, sampleSize)

					// Randomly sample.
					for i, _ := range sample {
						j := rand.Intn(num)
						sample[i] = verrs[j]
					}
				} else {
					sample = verrs
				}

				sstrings := make([]string, len(sample))

				for i, ve := range sample {
					if ve.Context != nil {
						sstrings[i] = fmt.Sprintf("line %d: `%s` %s", ve.Line, ve.Value, ve.Context)
					} else {
						sstrings[i] = fmt.Sprintf("line %d: `%s`", ve.Line, ve.Value)
					}
				}

				tw.Append([]string{
					f,
					fmt.Sprint(err.Code),
					err.Description,
					fmt.Sprint(num),
					strings.Join(errLineSteps(verrs), ", "),
					strings.Join(sstrings, "\n"),
				})
			}
		}

		if nerrs > 0 {
			hasErrors = true
			fmt.Println("* Field-level issues were found.")
			tw.Render()
		} else if len(lerrs) == 0 {
			fmt.Println("* Everything looks good!")
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// Returns a slice of line ranges that errors have occurred on.
func errLineSteps(errs []*validator.ValidationError) []string {
	var (
		start, end int
		steps      []string
	)

	for _, err := range errs {
		if start == 0 {
			start = err.Line
			end = start
			continue
		}

		if err.Line == start+1 {
			end = err.Line
			continue
		}

		// Skipped a line, log the step
		if start == end {
			steps = append(steps, fmt.Sprint(start))
		} else {
			steps = append(steps, fmt.Sprintf("%d-%d", start, end))
		}

		start = err.Line
		end = err.Line
	}

	// Skipped a line, log the step
	if start == end {
		steps = append(steps, fmt.Sprint(start))
	} else {
		steps = append(steps, fmt.Sprintf("%d-%d", start, end))
	}

	return steps
}
