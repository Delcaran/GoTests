package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"regexp"

	_ "github.com/mattn/go-sqlite3"
	csvmap "github.com/recursionpharma/go-csv-map"
)

func createDatabase(filePath string) {
	os.Remove(filePath)
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}
	defer db.Close()

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS Soci (
		ID INTEGER,
		Cognome TEXT,
		Nome TEXT,
		CodiceFiscale TEXT,
		Telefono TEXT,
		Email TEXT,
		Residenza TEXT,
		Domicilio TEXT,
		DataDiNascita TEXT,
		PRIMARY KEY(Cognome, Nome)
	);
	CREATE TABLE IF NOT EXISTS Documenti (
		Socio INTEGER,
		DataIscrizione TEXT,
		AnnoIscrizione INTEGER,
		DataScadenzaCertificatoMedico TEXT,
		CSEN TEXT,
		FOREIGN KEY(Socio) REFERENCES Soci(ID)
	);
	CREATE TABLE IF NOT EXISTS Presenze (
		Socio INTEGER,
		Sala TEXT,
		Data TEXT,
		Sparring INTEGER,
		FOREIGN KEY(Socio) REFERENCES Soci(ID)
	);
	CREATE TABLE IF NOT EXISTS Quote (
		Socio INTEGER,
		Data TEXT,
		Mese INTEGER,
		Tipo TEXT,
		FOREIGN KEY(Socio) REFERENCES Soci(ID)
	);

	CREATE VIEW IF NOT EXISTS NonIscritti AS SELECT
	Cognome,
	Nome
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND Documenti.DataIscrizione='';

	CREATE VIEW IF NOT EXISTS Iscritti AS SELECT
	Cognome,
	Nome,
	DataIscrizione
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND Documenti.DataIscrizione<>''
	ORDER BY DataIscrizione, Cognome, Nome;

	CREATE VIEW IF NOT EXISTS Votanti AS SELECT
	Cognome,
	Nome,
	DataIscrizione
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND ((Documenti.AnnoIscrizione=1 AND Documenti.DataIscrizione<=date('now', '-3 months')) OR Documenti.AnnoIscrizione>1)
	ORDER BY DataIscrizione, Cognome, Nome;

	CREATE VIEW IF NOT EXISTS NonCertificati AS SELECT
	Cognome,
	Nome,
	DataScadenzaCertificatoMedico
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND Documenti.DataScadenzaCertificatoMedico<=date('now')
	ORDER BY DataScadenzaCertificatoMedico ASC;

	CREATE VIEW IF NOT EXISTS CertificatiInScadenza AS SELECT
	Cognome,
	Nome,
	DataScadenzaCertificatoMedico
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND Documenti.DataScadenzaCertificatoMedico<=date('now', '+1 months')
	ORDER BY DataScadenzaCertificatoMedico ASC;

	CREATE VIEW IF NOT EXISTS TotPresenze AS SELECT
	Data,
	Sala,
	Sparring,
	COUNT(Socio)
	FROM Presenze
	GROUP BY Data
	ORDER BY Data ASC;

	CREATE VIEW IF NOT EXISTS IscrizioniPagate AS SELECT
	Socio
	FROM Quote
	WHERE tipo="Iscrizione";

	CREATE VIEW IF NOT EXISTS NonIscritti AS SELECT
	Soci.Cognome,
	Soci.Nome
	FROM Quote
	INNER JOIN Soci ON Soci.ID=Quote.Socio
	WHERE Quote.Socio NOT IN IscrizioniPagate;

	CREATE VIEW IF NOT EXISTS PresenzeSoci AS
	select soci.ID, soci.Cognome, soci.Nome, count(soci.ID) as presenze, strftime('%m', presenze.Data) as Mese
	from presenze
	inner join soci on soci.ID=presenze.Socio
	group by soci.ID, Mese;

	CREATE VIEW IF NOT EXISTS PresenzeSparring AS
	select soci.ID, soci.Cognome, soci.Nome, count(soci.ID) as presenze, strftime('%m', presenze.Data) as Mese
	from presenze
	inner join soci on soci.ID=presenze.Socio
	where presenze.Sparring=1
	group by soci.ID, Mese;

	CREATE VIEW IF NOT EXISTS PresenzeLezioni AS
	select soci.ID, soci.Cognome, soci.Nome, count(soci.ID) as presenze, strftime('%m', presenze.Data) as Mese
	from presenze
	inner join soci on soci.ID=presenze.Socio
	where presenze.Sparring=0
	group by soci.ID, Mese;

	CREATE VIEW IF NOT EXISTS QuotePagateMese AS
	select PresenzeSoci.ID, PresenzeSoci.Mese as Mese, PresenzeSoci.presenze, CASE WHEN Quote.Mese IS NOT NULL THEN 'SI' ELSE 'NO' END AS Pagato from PresenzeSoci
	left outer join Quote on Quote.Socio=PresenzeSoci.ID and Quote.Mese=PresenzeSoci.Mese
	where PresenzeSoci.ID not in (select Quote.Socio from Quote where Mese=0)
	order by PresenzeSoci.ID;

	CREATE VIEW IF NOT EXISTS QuoteDovute AS
	Select Soci.Cognome, Soci.Nome, QuotePagateMese.Mese, PresenzeSparring.Presenze as sparring, PresenzeLezioni.Presenze as lezioni
	from QuotePagateMese
	left outer join PresenzeSparring on QuotePagateMese.ID=PresenzeSparring.ID and QuotePagateMese.Mese=PresenzeSparring.Mese
	left outer join PresenzeLezioni on QuotePagateMese.ID=PresenzeLezioni.ID and QuotePagateMese.Mese=PresenzeLezioni.Mese
	inner join Soci on Soci.ID=QuotePagateMese.ID
	where QuotePagateMese.Pagato='NO'
	order by Soci.Cognome, Soci.Nome, QuotePagateMese.Mese;
	`

	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
}

func readSoci(dbPath string) map[string]int {
	// Load a csv file.
	f, _ := os.Open("Soci AS1920.csv")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmtSoci, err := tx.Prepare("INSERT INTO Soci (ID, Cognome, Nome, CodiceFiscale, Telefono, Email, Residenza, Domicilio, DataDiNascita) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtSoci.Close()
	stmtDocumenti, err := tx.Prepare("INSERT INTO Documenti (Socio, DataIscrizione, AnnoIscrizione, DataScadenzaCertificatoMedico, CSEN) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmtDocumenti.Close()

	// Create a new reader.
	r := csvmap.NewReader(bufio.NewReader(f))
	r.Columns, err = r.ReadHeader()
	if err != nil {
		log.Fatal(err)
	}
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	ids := make(map[string]int)
	for numRiga, riga := range records {
		var residenza, domicilio strings.Builder
		fmt.Fprintf(&residenza, "%s, %s %s (%s)", riga["Indirizzo"], riga["CAP"], riga["Citta"], riga["Provincia"])
		if len(riga["Domicilio"]) > 0 {
			fmt.Fprintf(&domicilio, "%s", riga["Domicilio"])
		} else {
			fmt.Fprintf(&domicilio, "%s", residenza.String())
		}

		ids[riga["Cognome"]] = numRiga + 1
		_, err = stmtSoci.Exec(numRiga+1, riga["Cognome"], riga["Nome"], strings.ToUpper(riga["Codice Fiscale"]), riga["Telefono"], strings.ToLower(riga["EMail"]), residenza.String(), domicilio.String(), riga["Data di Nascita"])
		if err != nil {
			log.Fatal(err)
		}

		var iscrizione string
		if len(riga["Iscritto"]) > 0 {
			data := strings.Split(riga["Iscritto"], "/")
			iscrizione = fmt.Sprintf("%s-%s-%s", data[2], data[1], data[0])
		}
		var scadenza string
		if len(riga["Certificato"]) > 0 {
			data := strings.Split(riga["Certificato"], "/")
			scadenza = fmt.Sprintf("%s-%s-%s", data[2], data[1], data[0])
		}
		_, err = stmtDocumenti.Exec(numRiga+1, iscrizione, riga["Anno"], scadenza, riga["Tessera CSEN"])
		if err != nil {
			log.Fatal(err)
		}
	}
	tx.Commit()
	return ids
}

func readPresenze(dbPath string, ids map[string]int) {
	// Load a csv file.
	f, _ := os.Open("Presenze AS1920.csv")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO Presenze (Socio, Sala, Data, Sparring) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	// Create a new reader.
	r := csv.NewReader(bufio.NewReader(f))
	count := 0
	date := make([]string, 1)
	sala := make([]string, 1)
	sparring := make([]string, 1)
	for {
		record, err := r.Read()
		// Stop at EOF.
		if err == io.EOF {
			break
		}
		switch count {
		case 0:
			for column := 2; column < len(record); column++ {
				data := strings.Split(record[column], " ")
				day := data[1]
				if data[0] == "lun" {
					sparring = append(sparring, "1")
				} else {
					sparring = append(sparring, "0")
				}
				year := "2019"
				month := "10"
				switch data[2] {
				case "ott":
					year = "2019"
					month = "10"
				case "nov":
					year = "2019"
					month = "11"
				case "dic":
					year = "2019"
					month = "12"
				case "gen":
					year = "2020"
					month = "01"
				case "feb":
					year = "2020"
					month = "02"
				case "mar":
					year = "2020"
					month = "03"
				case "apr":
					year = "2020"
					month = "04"
				case "mag":
					year = "2020"
					month = "05"
				case "giu":
					year = "2020"
					month = "06"
				}
				date = append(date, fmt.Sprintf("%s-%s-%s", year, month, day))
			}
		case 1:
			for column := 2; column < len(record); column++ {
				sala = append(sala, record[column])
			}
		default:
			for column := 2; column < len(record); column++ {
				if len(record[column]) > 0 {
					cognome := record[0]
					if id, ok := ids[cognome]; ok {
						_, err = stmt.Exec(id, sala[column], date[column], sparring[column])
						if err != nil {
							log.Fatal(err)
						}
					} else {
						fmt.Printf("Cognome non trovato: %s\n", cognome)
					}
				}
			}
		}
		count++
	}
	tx.Commit()
}

func readFinanze(dbPath string, anno string, ids map[string]int) {
	file, err := os.Open("registro.bean")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	const lineSep = "|"

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	blocchi := make([]string, 0)
	var buffer bytes.Buffer
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 {
			if len(buffer.String()) > 0 {
				buffer.WriteString(lineSep)
			}
			buffer.WriteString(line)
		} else {
			if len(buffer.String()) > 0 {
				blocchi = append(blocchi, buffer.String())
				buffer.Reset()
			}
		}
	}
	if len(buffer.String()) > 0 {
		blocchi = append(blocchi, buffer.String())
		buffer.Reset()
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO Quote (Socio, Data, Mese, Tipo) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	// ora da tutti i blocchi rimuovo quelli "non utili"
	for _, blocco := range blocchi {
		if strings.Contains(blocco, anno) && (strings.Contains(blocco, "#iscrizione") || strings.Contains(blocco, "quota:")) && !strings.Contains(blocco, " balance ") {
			var cognome string
			var mesi []string
			var tipo string
			var data string
			for numRiga, riga := range strings.Split(blocco, lineSep) {
				if len(riga) == 0 {
					continue
				}
				if numRiga == 0 {
					r := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\s\S\s"(.+)"\s".+"\s*(.*)`)
					matches := r.FindStringSubmatch(riga)
					if len(matches) > 1 {
						data = matches[1]
						cognome = matches[2]
						if len(matches) == 4 {
							tag := matches[3]
							if strings.Compare(tag, "#iscrizione") == 0 {
								if id, ok := ids[cognome]; ok {
									_, err = stmt.Exec(id, data, "NULL", "Iscrizione")
									if err != nil {
										log.Fatal(err)
									}
								} else {
									fmt.Printf("Cognome non trovato: %s\n", cognome)
								}
							}
						}
					}
				} else {
					riga := strings.TrimSpace(riga)
					if strings.HasPrefix(riga, "quota:") {
						mesi = strings.Split(strings.Trim(strings.Fields(riga)[1], "\""), ",")
					}
					if strings.HasPrefix(riga, "tipo:") {
						tipo = strings.Trim(strings.Fields(riga)[1], "\"")
					}
				}
				numRiga++
			}
			if len(data) > 0 && len(cognome) > 0 && (len(mesi) > 0 || len(tipo) > 0) {
				for _, mese := range mesi {
					mesenum := 0
					switch mese {
					case "anno":
						mesenum = 0
					case "ottobre":
						mesenum = 10
					case "novembre":
						mesenum = 11
					case "dicembre":
						mesenum = 12
					case "gennaio":
						mesenum = 1
					case "febbraio":
						mesenum = 2
					case "marzo":
						mesenum = 3
					case "aprile":
						mesenum = 4
					case "maggio":
						mesenum = 5
					case "giugno":
						mesenum = 6
					}
					if id, ok := ids[cognome]; ok {
						_, err = stmt.Exec(id, data, mesenum, tipo)
						if err != nil {
							log.Fatal(err)
						}
					} else {
						fmt.Printf("Cognome non trovato: %s\n", cognome)
					}
				}
			}
		}
	}
	tx.Commit()
}

func main() {
	dbPath := "Database.db"
	createDatabase(dbPath)
	ids := readSoci(dbPath)
	readPresenze(dbPath, ids)
	readFinanze(dbPath, "2019", ids)
}
