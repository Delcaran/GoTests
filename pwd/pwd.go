package main

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"time"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	type Entry struct {
		Account string `xml:"account,attr"`
		User  string
		Pass  string
	}
	type Result struct {
		XMLName xml.Name `xml:"Database"`
		Entry   []Entry
	}
	var database Result

	// Open our xmlFile
	xmlFile, err := os.Open("database.xml")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Successfully Opened database.xml")
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	/*
	data := `
	<Database>
		<Entry account="nome_account">
			<User>Example Inc.</User>
			<Pass>password</Pass>
		</Entry>
		<Entry account="nome_account2">
			<User>Example Inc. 2 </User>
			<Pass>password 2 </Pass>
		</Entry>
	</Database>
	`
	*/

	data, _ := ioutil.ReadAll(xmlFile)
	err = xml.Unmarshal(data, &database)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	fmt.Printf("XMLName: %#v\n", database.XMLName)

	for i := 0; i < len(database.Entry); i += 1 {
		v := database.Entry[i]
		//fmt.Printf("Account  : %v\n", v.Account)
		//fmt.Printf("User     : %v\n", v.User)
		//fmt.Printf("Password : %v\n", v.Pass)

		data := []byte(v.Pass)
		hash := fmt.Sprintf("%X", sha1.Sum(data))
		short := hash[:5]

		res, err := http.Get("https://api.pwnedpasswords.com/range/" + short)
		if err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s", body)

		time.Sleep(2000 * time.Millisecond)
	}
}
