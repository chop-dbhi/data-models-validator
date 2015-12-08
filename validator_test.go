package validator

import (
	"bytes"
	"io"
	"testing"

	"github.com/chop-dbhi/data-models-service/client"
)

var (
	header = `"C_HLEVEL","C_FULLNAME","C_NAME","C_SYNONYM_CD","C_VISUALATTRIBUTES","C_TOTALNUM","C_BASECODE","C_METADATAXML","C_FACTTABLECOLUMN","C_TABLENAME","C_COLUMNNAME","C_COLUMNDATATYPE","C_OPERATOR","C_DIMCODE","C_COMMENT","C_TOOLTIP","M_APPLIED_PATH","UPDATE_DATE","DOWNLOAD_DATE","IMPORT_DATE","SOURCESYSTEM_CD","VALUETYPE_CD","M_EXCLUSION_CD","C_PATH","C_SYMBOL"` + "\n"

	line = `"3","\PCORI\VITAL\TOBACCO\SMOKING\","Smoked Tobacco","N","FAE",,,,"concept_cd","CONCEPT_DIMENSION","concept_path","T","like","\PCORI\VITAL\TOBACCO\SMOKING\","CDMv2","This field is new to v3.0. Indicator for any form of tobacco that is smoked.Per Meaningful Use guidance, smoking status includes any form of tobacco that is smoked, but not all tobacco use. ""Light smoker"" is interpreted to mean less than 10 cigarettes per day, or an equivalent (but less concretely defined) quantity of cigar or pipe smoke. ""Heavy smoker"" is interpreted to mean greater than 10 cigarettes per day or an equivalent (but less concretely defined) quantity of cigar or pipe smoke. ","@","2015-08-20 312:14:14.0","2015-08-20 12:14:14.0","2015-08-20 12:14:14.0","PCORNET_CDM",,,"\PCORI\VITAL\TOBACCO\","SMOKING"` + "\n"

	table *client.Table
)

func init() {
	c, _ := client.New("http://data-models.origins.link")
	model, _ := c.ModelRevision("i2b2_pedsnet", "2.0.0")
	table = model.Tables.Get("i2b2")
}

type streamReader struct {
	Size uint

	header []byte
	line   []byte

	readHead    bool
	lineCounter uint
}

func (r *streamReader) Read(b []byte) (int, error) {
	// Return header
	if !r.readHead {
		r.readHead = true
		return copy(b, r.header), nil
	}

	// Exit once the size has been reached.
	if r.lineCounter == r.Size {
		return 0, io.EOF
	}

	r.lineCounter++
	n := copy(b, r.line)

	return n, nil
}

func newStreamReader(size uint) *streamReader {
	return &streamReader{
		Size:   size,
		header: []byte(header),
		line:   []byte(line),
	}
}

func TestParseCSVLine(t *testing.T) {
	r := bytes.NewBuffer([]byte(line))
	record := make([]string, 25)

	col, err := parseCSVLine(r, record)

	if err != nil {
		t.Fatal(err)
	}

	if col != 25 {
		t.Errorf("expected column 25, got %d", col)
	}

	if record[24] != "SMOKING" {
		t.Errorf("expected last element to be `SMOKING`, got `%s`", record[24])
	}
}

func BenchmarkParseCSVLine(b *testing.B) {
	buf := []byte(line)
	r := bytes.NewBuffer(buf)
	rec := [25]string{}

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		parseCSVLine(r, rec[:0])

		b.StopTimer()
		r = bytes.NewBuffer(buf)
	}
}

func BenchmarkValidateRow(b *testing.B) {
	r := bytes.NewBuffer(nil)
	r.Write([]byte(header))
	r.Write([]byte(line))

	v := New(r, table)
	v.Init()

	buf := []byte(line)
	r = bytes.NewBuffer(buf)
	row := make([]string, 25)
	parseCSVLine(r, row)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.validateRow(row)
	}
}
