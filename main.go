package main

import (
	"fmt"
	"encoding/csv"
	"encoding/json"
	"flag"
	"github.com/elzapp/go-ofx"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type CsvField struct {
	FieldRef   int        `json:"field"`
	DateFormat string     `json:"dateformat"`
	Sum        []CsvField `json:"sum"`
	Multiplier float64    `json:"multiplier"`
}

func (f CsvField) DateValue(row []string) (dt time.Time, err error) {
	dt, err = time.Parse(f.DateFormat, row[f.FieldRef])
	return dt, err
}
func (f CsvField) StringValue(row []string) (s string) {
	s = row[f.FieldRef]
	return s
}

func (f CsvField) FloatValue(row []string) (sum float64) {
	for _, el := range f.Sum {
		sum = sum + el.FloatValue(row)
	}
	if len(f.Sum) == 0 {
		literalValue, err := strconv.ParseFloat(strings.Replace(row[f.FieldRef], ",", ".", 1), 64)
		if err != nil {
			literalValue = 0.0
		}
		sum = sum + (literalValue * f.Multiplier)
	}
	return
}

type CsvFieldSpec struct {
	Trntype  CsvField `json:"trntype"`
	Dtposted CsvField `json:"dtposted"`
	Dtuser   CsvField `json:"dtuser"`
	Trnamt   CsvField `json:"trnamt"`
	Memo     CsvField `json:"memo"`
	Ftid     CsvField `json:"ftid"`
}

type CsvFormat struct {
	Name   string       `json:"name"`
	Curdef string       `json:"curdef"`
	Header int64        `json:"skip-header"`
	Footer int64        `json:"skip-footer"`
	Fields CsvFieldSpec `json:"fields"`
	Comma string `json:"comma"`
}

type CsvSpec struct {
	Format []CsvFormat `json:"format"`
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func decodeRecord(record []string, format CsvFormat) ofx.OfxTransaction {
	var tx ofx.BankTransaction
	am := format.Fields.Trnamt.FloatValue(record)
	tx.Amount = am
	tx.PostedDate, _ = format.Fields.Dtposted.DateValue(record)
	tx.InterestDate, _ = format.Fields.Dtuser.DateValue(record)
	tx.Memo = format.Fields.Memo.StringValue(record)
	return tx.ToOfx()
}

type StringWriter interface {
    WriteString(s string) (n int, err error)
}

func convert(reader io.Reader, writer io.Writer, logger StringWriter, format CsvFormat) {
	r := csv.NewReader(reader)
	r.Comma = []rune(format.Comma)[0]
	r.Read()

	var ofxlist ofx.OfxTransactionList
	ofxlist.CurDef = "NOK"
	ofxlist.PayerAccount = "0000000000"
	ofxlist.PayerBank = "Sbanken"
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		tx := decodeRecord(record, format)
		ofxlist.Transactions = append(ofxlist.Transactions, tx)
	}
	logger.WriteString(fmt.Sprintln(ofxlist))
	ofxlist.WriteOFX(writer)
}

func readConfig(configFilename string) (csvSpec CsvSpec){
	dat, err := ioutil.ReadFile("specs.json")
	check(err)
	json.Unmarshal(dat, &csvSpec)
	return
}

func findFormat(spec CsvSpec, formatname string) (format CsvFormat) {

	for _, f := range spec.Format {
		if f.Name == formatname {
			return f
		}
	}

	format = spec.Format[0]
	return
}


func getReader(source string) (reader io.Reader, file os.File) {
	reader = os.Stdin
	if(source != "-"){
		file, _ := os.Open(source)
		reader = file
	}
	return
}

func getWriter(source string) (reader io.Writer, file os.File) {
	reader = os.Stdout
	if(source != "-"){
		file, _ := os.Create(source)
		reader = file
	}
	return
}


func main() {
	var source string
	var destination string
	var formatname string
	var configFilename string
	flag.StringVar(&formatname, "format", "spv-kreditt", "Name of format to parse as, defined in the config.json")
	flag.StringVar(&configFilename, "config", "specs.json", "Config file to load")
	flag.StringVar(&source, "source", "-", "csv file to read, use '-' for stdin")
	flag.StringVar(&destination, "destination", "-", "ofx file to write, use '-' for stdin")

	flag.Parse()
	os.Stderr.WriteString("Source: "+source+"\n")
	os.Stderr.WriteString("Format: "+formatname+"\n")
	os.Stderr.WriteString("Destination: "+destination+"\n")
	os.Stderr.WriteString("Config: "+configFilename+"\n")

	config := readConfig(configFilename)
	format := findFormat(config, formatname)
	//var writer io.Writer = os.Stdout

	reader, rf := getReader(source)
	defer rf.Close()
	writer, wf := getWriter(destination)
	wf.Close()


	convert(reader, writer, os.Stderr, format)
}
