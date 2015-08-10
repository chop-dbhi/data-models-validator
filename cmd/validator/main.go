package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/chop-dbhi/data-models-service/client"
	"github.com/chop-dbhi/data-models-validator"
)

const DataModelsService = "http://data-models.origins.link"

var usage = `usage: data-models-validator [-model <model>]
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
to be validated against. If not specified, the header will be used to detect
which table the file corresponds to. The only time an explicit table should need
to be used is if two tables have the same set of fields making the detection
ambiguous.

Examples:

	# Validate person.csv file against the OMOP v5 data model.
	data-models-validator -model omop -version 5.0.0 person.csv

	# Validate foo.csv against the person table in the OMOP v5 data model.
	data-models-validator -model omop -version 5.0.0 foo.csv:person

	# Validate the STDIN stream denoting it is tab-delimited and gzipped.
	data-models-validator -model omop -version 5.0.0 -delim $'\t' -compr gzip
`

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
		var versions []string

		for _, model = range revisions.List() {
			if model.Version == version {
				break
			}

			versions = append(versions, model.Version)
		}

		if model == nil {
			fmt.Printf("model %s has versions: %s\n", model, strings.Join(versions, ", "))
			os.Exit(1)
		}
	}

	fmt.Printf("Model: %s\n", model.Name)
	fmt.Printf("Version: %s\n", model.Version)

	var (
		tableName string
		table     *client.Table
	)

	inputs := flag.Args()

	for _, name := range inputs {
		// The file name may have a suffix containing the table name, name[:table].
		// The fallback is to use the file name without the extension.
		toks := strings.SplitN(name, ":", 2)

		if len(toks) == 2 {
			name = toks[0]
			tableName = toks[1]
		} else {
			name = toks[0]

			toks = strings.SplitN(name, ".", 2)
			tableName = toks[0]
		}

		if tableName == "" {
			fmt.Println("error: table not specified")
			continue
		}

		if table = model.Tables.Get(tableName); table == nil {
			fmt.Printf("error: unknown table %s. choices are: %s\n", tableName, strings.Join(model.Tables.Names(), ", "))
			continue
		}

		// Open the reader.
		reader, err := validator.Open(name, compr)

		if err != nil {
			fmt.Println(err)
			continue
		}

		defer reader.Close()

		v := validator.New(reader, table)

		if err = v.Init(); err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		fmt.Printf("Table: %s\n", table.Name)

		// v.Plan.Print(os.Stdout)

		if err = v.Run(); err != nil {
			fmt.Printf("error: %v\n", err)
		}

		// Print the results.
		result := v.Result()

		fmt.Println("Result:")

		var errmap map[*validator.Error][]*validator.ValidationError

		tw := tabwriter.NewWriter(os.Stdout, 0, 8, 4, '\t', tabwriter.AlignRight)

		// Output the error occurrence per field.
		for _, f := range v.Header {
			errmap = result.FieldErrors(f)

			if len(errmap) == 0 {
				continue
			}

			fmt.Fprintf(tw, "%s:\n", f)

			for err, verrs := range errmap {
				fmt.Fprintf(tw, "\t- %v\t%d\n", err, len(verrs))
			}
		}

		tw.Flush()
	}
}
