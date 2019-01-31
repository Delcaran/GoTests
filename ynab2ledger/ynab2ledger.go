package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
)

type Record struct {
	Account        string
	Flag           string
	CheckNumber    string
	Data           string
	Payee          string
	Category       string
	MasterCategory string
	SubCategory    string
	Memo           string
	Outflow        string
	Inflow         string
	Cleared        string
	Bilancio       string
}

func (entry *Record) fill(record []string) {
	if len(record) > 0 {
		entry.Account = record[0]
		entry.Flag = record[1]
		entry.CheckNumber = record[2]
		entry.Data = record[3]
		entry.Payee = record[4]
		entry.Category = record[5]
		entry.MasterCategory = record[6]
		entry.SubCategory = record[7]
		entry.Memo = record[8]
		entry.Outflow = record[9]
		entry.Inflow = record[10]
		entry.Cleared = record[11]
		entry.Bilancio = record[12]
	}
}

func (entry Record) print() {
	fmt.Println("----------------------")
	fmt.Println("Account     : " + entry.Account)
	fmt.Println("Flag        : " + entry.Flag)
	fmt.Println("CheckNumber : " + entry.CheckNumber)
	fmt.Println("Data        : " + entry.Data)
	fmt.Println("Payee       : " + entry.Payee)
	fmt.Println("Category    : " + entry.Category)
	fmt.Println("Memo        : " + entry.Memo)
	fmt.Println("Outflow     : " + entry.Outflow)
	fmt.Println("Inflow      : " + entry.Inflow)
	fmt.Println("Cleared     : " + entry.Cleared)
	fmt.Println("Bilancio    : " + entry.Bilancio)
}

func parseEuro(stringa string) string {
	if strings.HasPrefix(stringa, "€") {
		return strings.Replace(strings.Split(stringa, "€")[1], ",", ".", -1)
	}
	return stringa
}

func (entry Record) parse() {
	type Params struct {
		Date           string
		Check          string
		Cleared        string
		Description    string
		Payee          string
		AccountEntrata string
		AccountUscita  string
		Entrata        string
		Uscita         string
		Bilancio       string
	}
	var output bytes.Buffer
	params := &Params{
		Date:     entry.Data,
		Check:    entry.CheckNumber,
		Bilancio: parseEuro(entry.Bilancio),
	}

	switch entry.Cleared {
	case "U":
		params.Cleared = ""
	case "R":
		params.Cleared = "*"
	}
	if len(entry.Memo) > 0 {
		params.Description = entry.Memo
	} else {
		params.Description = entry.Payee
	}
	if strings.HasPrefix(entry.Payee, "Transfer") {
		params.Payee = ""
		params.AccountEntrata = strings.Split(entry.Payee, "Transfer : ")[1]
	} else {
		if params.Description != entry.Payee {
			params.Payee = "\n\t; Payee: " + entry.Payee
		} else {
			params.Payee = ""
		}
		params.AccountEntrata = entry.Category
	}
	if strings.HasPrefix(entry.Memo, "Income") {
		params.AccountUscita = "Entrate:" + entry.Payee
		params.AccountEntrata = entry.Account
		params.Entrata = parseEuro(entry.Inflow)
		params.Uscita = params.Entrata
	} else {
		params.AccountUscita = entry.Account
		params.AccountEntrata = "Uscite:" + entry.Category
		params.Uscita = parseEuro(entry.Outflow)
		params.Entrata = params.Uscita
	}

	transaction_description := "{{.Date}} {{.Check}} {{.Cleared}} {{.Description}}{{.Payee}}"
	transaction_to := "\t{{.AccountEntrata}}\t€{{.Entrata}}"
	transaction_from := "\t{{.AccountUscita}}\t€-{{.Uscita}} = {{.Bilancio}}"
	full_transaction := strings.Join([]string{transaction_description, transaction_to, transaction_from, "\n"}, "\n")
	tmpl, _ := template.New("output").Parse(full_transaction)
	tmpl.Execute(&output, params)
	output.WriteTo(os.Stdout)
}

func ReadCsvFile(filePath string) {
	f, _ := os.Open(filePath)
	r := csv.NewReader(bufio.NewReader(f))
	r.Comma = '\t'
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		var entry Record
		entry.fill(record)
		//entry.print()
		entry.parse()

		/*
			for value := range record {
				fmt.Printf("%d :  %v\n", value, record[value])
			}
		*/

	}
}

func main() {
	ReadCsvFile("A:/_sync/_temp/ledger/register.csv")
}
