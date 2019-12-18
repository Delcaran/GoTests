package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	csvmap "github.com/recursionpharma/go-csv-map"
)

type emailConfig struct {
	Username string
	Password string
	Host     string
	Port     int
}

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
	Nome,
	Email
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
	Email,
	DataScadenzaCertificatoMedico
	FROM Soci
	INNER JOIN Documenti ON Soci.ID=Documenti.Socio AND Documenti.DataScadenzaCertificatoMedico<=date('now')
	ORDER BY DataScadenzaCertificatoMedico ASC;

	CREATE VIEW IF NOT EXISTS CertificatiInScadenza AS SELECT
	Cognome,
	Nome,
	Email,
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
	Soci.Nome,
	Soci.Email
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
	Select Soci.Cognome, Soci.Nome, Soci.Email, QuotePagateMese.Mese, PresenzeSparring.Presenze as sparring, PresenzeLezioni.Presenze as lezioni
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

func readSoci(filePath string, dbPath string) map[string]int {
	// Load a csv file.
	f, _ := os.Open(filePath)

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

func readPresenze(filePath string, dbPath string, ids map[string]int) {
	// Load a csv file.
	f, _ := os.Open(filePath)

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

func readFinanze(filePath string, dbPath string, ids map[string]int) {
	file, err := os.Open(filePath)
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
		if (strings.Contains(blocco, "#iscrizione") || strings.Contains(blocco, "quota:")) && !strings.Contains(blocco, " balance ") {
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

func reportCertificati(dbPath string, now time.Time) string {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, errRow := db.Query("SELECT * FROM CertificatiInScadenza ORDER BY Cognome, Nome, DataScadenzaCertificatoMedico")
	if errRow != nil {
		log.Fatalln(errRow)
	}
	defer rows.Close()

	currYear, currMonth, currDay := now.Date()
	dataOggi := time.Date(currYear, currMonth, currDay, 0, 0, 0, 0, now.Location())

	var outputMissing string
	var outputScaduti string
	var outputScadenti string
	for rows.Next() {
		var cognome string
		var nome string
		var email string
		var data string
		rows.Scan(&cognome, &nome, &email, &data)
		if len(data) > 0 {
			var day int
			var month int
			var year int
			fmt.Sscanf(data, "%d-%d-%d", &year, &month, &day)
			dataCertificato := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
			txt := fmt.Sprintf("- %s %s: %d/%d/%d\r\n", cognome, nome, day, month, year)
			if dataCertificato.Before(dataOggi) {
				outputScaduti = outputScaduti + txt
			} else {
				outputScadenti = outputScadenti + txt
			}
		} else {
			outputMissing = outputMissing + fmt.Sprintf("- %s %s\r\n", cognome, nome)
		}
	}
	rows.Close()

	output := fmt.Sprintf("PROBLEMI CERTIFICATI AL %d/%d/%d", currDay, currMonth, currYear)
	output = fmt.Sprintf("%s\r\n%s\r\n\r\n", output, strings.Repeat("=", len(output)))
	if (len(outputMissing) + len(outputScaduti) + len(outputScadenti)) > 0 {
		if len(outputMissing) > 0 {
			output = output + fmt.Sprintf("## SENZA CERTIFICATO\r\n\r\n%s\r\n", outputMissing)
		}
		if len(outputScaduti) > 0 {
			output = output + fmt.Sprintf("## SCADUTI\r\n\r\n%s\r\n", outputScaduti)
		}
		if len(outputScadenti) > 0 {
			output = output + fmt.Sprintf("## IN SCADENZA\r\n\r\n%s\r\n", outputScadenti)
		}
	} else {
		output = output + fmt.Sprintln("Tutti in regola, nulla da segnalare.")
	}

	return output
}

func monthToName(m int) string {
	var months = [...]string{"Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno", "Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre"}
	return months[m-1]
}

func reportQuote(dbPath string, now time.Time) string {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, errRow := db.Query("SELECT * FROM QuoteDovute ORDER BY Cognome, Nome, Mese")
	if errRow != nil {
		log.Fatalln(errRow)
	}
	defer rows.Close()

	var outputSoci string
	var outputSocio string
	var currentCognome string
	for rows.Next() {
		var cognome string
		var nome string
		var mese int
		var email string
		var sparring sql.NullInt32
		var lezioni sql.NullInt32
		rows.Scan(&cognome, &nome, &email, &mese, &sparring, &lezioni)
		var meseStr = monthToName(mese)
		if currentCognome != cognome {
			outputSoci = outputSoci + fmt.Sprintf("%s\r\n", outputSocio)
			currentCognome = cognome
			outputSocio = fmt.Sprintf("+ %s %s:\r\n\t- %s", cognome, nome, meseStr)
		} else {
			outputSocio = outputSocio + fmt.Sprintf("\r\n\t- %s", meseStr)
		}
		if sparring.Valid {
			value, err := sparring.Value()
			if err != nil {
				log.Fatalln(err)
			}
			outputSocio = outputSocio + fmt.Sprintf("\r\n\t\tSparring: %d", value)
		}
		if lezioni.Valid {
			value, err := lezioni.Value()
			if err != nil {
				log.Fatalln(err)
			}
			outputSocio = outputSocio + fmt.Sprintf("\r\n\t\tLezioni : %d", value)
		}
	}
	rows.Close()

	currYear, currMonth, currDay := now.Date()
	output := fmt.Sprintf("QUOTE DA PAGARE AL %d/%d/%d", currDay, currMonth, currYear)
	output = fmt.Sprintf("%s\r\n%s\r\n", output, strings.Repeat("=", len(output)))
	if len(outputSoci) > 0 {
		output = output + outputSoci
	} else {
		output = output + fmt.Sprintln("Tutti in regola, nulla da segnalare.")
	}

	return output
}

func reportNonIscritti(dbPath string, now time.Time) string {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, errRow := db.Query("SELECT * FROM NonIscritti ORDER BY Cognome, Nome")
	if errRow != nil {
		log.Fatalln(errRow)
	}
	defer rows.Close()

	var outputData string
	for rows.Next() {
		var cognome string
		var nome string
		var email string
		rows.Scan(&cognome, &nome, &email)
		outputData = outputData + fmt.Sprintf("- %s %s\r\n", cognome, nome)
	}
	rows.Close()

	currYear, currMonth, currDay := now.Date()
	output := fmt.Sprintf("NON ISCRITTI AL %d/%d/%d", currDay, currMonth, currYear)
	output = fmt.Sprintf("%s\r\n%s\r\n\r\n", output, strings.Repeat("=", len(output)))
	if len(outputData) > 0 {
		output = output + outputData + "\r\n"
	} else {
		output = output + fmt.Sprintln("Tutti in regola, nulla da segnalare.")
	}

	return output
}

func sendMail(receivers []string, subject string, body string, toMembers bool) {
	sender := "saamfvg+reports@achillemarozzo.fvg.it"
	conf := &emailConfig{"username", "password", "smtp.gmail.com", 587}
	auth := smtp.PlainAuth("", conf.Username, conf.Password, conf.Host)
	msg := []byte("From: " + sender + "\r\n" +
		"To: " + strings.Join(receivers, ",") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")
	log.Println(string(msg))
	if toMembers {
		receivers = append(receivers, "condir@achillemarozzo.fvg.it") // BCC
	}
	err := smtp.SendMail(fmt.Sprintf("%s:%d", conf.Host, conf.Port), auth, sender, receivers, []byte(msg))
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	rootPath := "."
	if len(os.Args) > 1 {
		rootPath = os.Args[1]
	}
	baseDir := filepath.Join(rootPath, "Gestione di Sala")
	dbPath := filepath.Join(baseDir, "Database.db")
	registro := filepath.Join(baseDir, "Finanze", "AS20192020.bean")
	presenze := filepath.Join(baseDir, "Soci", "AS 19-20", "Presenze AS1920.csv")
	soci := filepath.Join(baseDir, "Soci", "AS 19-20", "Soci AS1920.csv")

	// Data loading

	createDatabase(dbPath)

	ids := readSoci(soci, dbPath)
	readPresenze(presenze, dbPath, ids)
	readFinanze(registro, dbPath, ids)

	// Data loaded, reporting
	adesso := time.Now()
	certificati := reportCertificati(dbPath, adesso)
	quote := reportQuote(dbPath, adesso)
	noniscritti := reportNonIscritti(dbPath, adesso)
	body := fmt.Sprintf("\r\n%s\r\n%s\r\n%s\r\n", noniscritti, certificati, quote)
	sendMail([]string{"condir@achillemarozzo.fvg.it"}, "Report situazione di Sala", body, false)
}
