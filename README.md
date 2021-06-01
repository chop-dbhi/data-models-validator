# Data Models Validator

[![Build Status](https://travis-ci.org/chop-dbhi/data-models-validator.svg?branch=master)](https://travis-ci.org/chop-dbhi/data-models-validator) [![GoDoc](https://godoc.org/github.com/chop-dbhi/data-models-validator?status.svg)](https://godoc.org/github.com/chop-dbhi/data-models-validator)

A validator for CSV files adhering to the [Data Models](https://github.com/chop-dbhi/data-models) specification.

## Install

Download the latest binary from the [releases page](https://github.com/chop-dbhi/data-models-validator/releases) for your architecture: Windows, Linux, or OS X. The following examples assume the binary has been placed on your `PATH` with the name `data-models-validator`.

## Usage

Run the validator by specifying the model and version of the data model you wish to check against and one or more input files (if the files are not named after the corresponding tables, you can specify the correct tables).

Validate person.csv file against the PEDSnet v2 data model:

```
$ data-models-validator -model pedsnet -version 2.0.0 person.csv
```

Validate foo.csv against the person table in the PEDSnet v2 data model:

```
$ data-models-validator -model pedsnet -version 2.0.0 foo.csv:person
```

Run the following to see the full usage:

```
$ data-models-validator -help
```

## Functionality

The validator checks the following:

- header matches fields of specified table
- each row of data has the correct number of fields
- data is encoded in UTF8
- quotes within data values are escaped
- date and datetime data is valid and properly formatted
- integer and number (float) data is valid and fits in 32-bit types
- required data is not left null
- string data does not exceed defined max lengths

The validator does **not** check:

- foreign key referential integrity
- data model conventions such as correct concept usage
- uniqueness of data across rows

## Known Bugs

If the validator is run several times in quick succession, an error from the underlying data models service is thrown:

```
$ data-models-validator -model i2b2_pedsnet -version 2.0.0 obs-fact.csv:observation_fact
error decoding model revisions: invalid character '<' looking for beginning of value
```

No harm is done to any data and by waiting several minutes before re-running, the problem can be circumvented. The development team is working on a fix ASAP and you can track our progress [here](https://github.com/chop-dbhi/data-models-validator/issues/9)

## Future Directions

Soon, we hope to add higher level validation checks, especially of referential integrity.

## Output Examples

If the filename does not match a table in the model, a list of known tables in the model is printed. The solution is to specify the table by adding `:table_name` to the end of your file name:

```
$ data-models-validator -model pedsnet -version 2.0.0 PEDSNET_DRUG_EXPOSURE.csv
Validating against model 'pedsnet/2.0.0'
* Unknown table 'PEDSNET_DRUG_EXPOSURE'.
Choices are: care_site, concept, concept_ancestor, concept_class, concept_relationship, concept_synonym, condition_occurrence, death, domain, drug_exposure, drug_strength, fact_relationship, location, measurement, observation, observation_period, person, procedure_occurrence, provider, relationship, source_to_concept_map, visit_occurrence, visit_payer, vocabulary

$ data-models-validator -model pedsnet -version 2.0.0 PEDSNET_DRUG_EXPOSURE.csv:drug_exposure
```

If the header does not match the expected set of fields, the expected and actual number of fields as well as any unknown and/or missing fields found in the header are printed:

```
$ data-models-validator -model pedsnet -version 2.0.0 original_visit_occurrence.csv:visit_occurrence
Validating against model 'pedsnet/2.0.0'
* Evaluating 'visit_occurrence' table in 'original_visit_occurrence.csv'...
* Problem reading CSV header: line 0: [code: 201] Header does not contain the correct set of fields: (expectedLength:12, actualLength:13, unknownFields:[visit_occurrence_source_id])
```

If the file passes validation, success is reported:

```
$ data-models-validator -model pedsnet -version 2.0.0 person.csv
Validating against model 'pedsnet/2.0.0'
* Evaluating 'person' table in 'person.csv'...
* Everything looks good!
```

If errors are found in the data, the errors are reported. For each field in which an error is found and each type of error in that field, the number of occurrences of the error and a small random sample of actual error values (prepended by line number) are shown:

```
$ data-models-validator -model pedsnet -version 2.0.0 measurement.csv
Validating against model 'pedsnet/2.0.0'
* Evaluating 'measurement' table in 'measurement.csv'...
* A few issues were found
+--------------------------+------+--------------------------------+-------------+--------------------------------+
|          FIELD           | CODE |             ERROR              | OCCURRENCES |            SAMPLES             |
+--------------------------+------+--------------------------------+-------------+--------------------------------+
| measurement_source_value |  300 | Value is required              |         159 | 96897086:'' 96920063:''        |
|                          |      |                                |             | 96973571:'' 96899225:''        |
|                          |      |                                |             | 96912743:''                    |
| unit_source_value        |  300 | Value is required              |     8938919 | 56172499:'' 84397591:''        |
|                          |      |                                |             | 64597721:'' 64982471:''        |
|                          |      |                                |             | 63311022:''                    |
| value_source_value       |  203 | Value contains bare double     |           7 | 96847421:'COMPLEXITY           |
|                          |      | quotes (")                     |             | CLINICAL LABORATORY            |
|                          |      |                                |             | TESTING."' 96847441:'"THIS     |
|                          |      |                                |             | TEST WAS DEVELOPED AND ITS     |
|                          |      |                                |             | PERFORMANCE CHARACTERISTICS'   |
|                          |      |                                |             | 64833023:'"X"=30.0%'           |
|                          |      |                                |             | 96847452:'"THIS TEST           |
|                          |      |                                |             | WAS DEVELOPED AND ITS          |
|                          |      |                                |             | PERFORMANCE CHARACTERISTICS'   |
|                          |      |                                |             | 64833023:'"X"=30.0%'           |
+--------------------------+------+--------------------------------+-------------+--------------------------------+
```

## Developers

Go 1.16+ is required.

Build a local binary.

```
make build
```

### Release

Create a git tag on the commit to be released. Then create a build for all targets.

```
make dist
```

This will create zip files in `dist/` that can be uploaded to the Github releases page for the tag.
