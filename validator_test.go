package validator

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/chop-dbhi/data-models-service/client"
)

var (
	header = `"C_HLEVEL","C_FULLNAME","C_NAME","C_SYNONYM_CD","C_VISUALATTRIBUTES","C_TOTALNUM","C_BASECODE","C_METADATAXML","C_FACTTABLECOLUMN","C_TABLENAME","C_COLUMNNAME","C_COLUMNDATATYPE","C_OPERATOR","C_DIMCODE","C_COMMENT","C_TOOLTIP","M_APPLIED_PATH","UPDATE_DATE","DOWNLOAD_DATE","IMPORT_DATE","SOURCESYSTEM_CD","VALUETYPE_CD","M_EXCLUSION_CD","C_PATH","C_SYMBOL"` + "\n"

	line = `"3","\PCORI\VITAL\TOBACCO\SMOKING\","Smoked Tobacco","N","FAE",,,,"concept_cd","CONCEPT_DIMENSION","concept_path","T","like","\PCORI\VITAL\TOBACCO\SMOKING\","CDMv2","This field is new to v3.0. Indicator for any form of tobacco that is smoked.Per Meaningful Use guidance, smoking status includes any form of tobacco that is smoked, but not all tobacco use. ""Light smoker"" is interpreted to mean less than 10 cigarettes per day, or an equivalent (but less concretely defined) quantity of cigar or pipe smoke. ""Heavy smoker"" is interpreted to mean greater than 10 cigarettes per day or an equivalent (but less concretely defined) quantity of cigar or pipe smoke. ","@","2015-08-20 312:14:14.0","2015-08-20 12:14:14.0","2015-08-20 12:14:14.0","PCORNET_CDM",,,"\PCORI\VITAL\TOBACCO\","SMOKING"` + "\n"

	table *client.Table
)

func init() {
	c, err := client.New("https://data-models-service.research.chop.edu")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing client: %s\n", err)
		os.Exit(1)
	}

	model, _ := c.ModelRevision("i2b2_pedsnet", "2.0.1")
	table = model.Tables.Get("i2b2")
}

func BenchmarkValidateRow(b *testing.B) {
	r := bytes.NewBuffer(nil)
	r.Write([]byte(header))
	r.Write([]byte(line))

	v := New(r, table)
	v.Init()

	buf := []byte(line)
	r = bytes.NewBuffer(buf)

	cr := DefaultCSVReader(r)
	row, _ := cr.Read()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.validateRow(row)
	}
}
