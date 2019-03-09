package main

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type singleEntry struct {
	Key   string
	Value string
}
type entry struct {
	UUID          string
	SingleEntries []singleEntry `xml:"String"`
}
type group struct {
	Name  string
	Entry []entry
	Group []group
}
type root struct {
	Group []group
}
type result struct {
	XMLName xml.Name `xml:"KeePassFile"`
	Root    []root
}

type data struct {
	Account  string
	Username string
	Password string
}

func (gruppo group) parseGroup() []data {
	//fmt.Printf("group: %v\n", gruppo.Group[j].Name)
	var informazioni []data
	for z := 0; z < len(gruppo.Entry); z++ {
		//fmt.Printf("UUID  : %v\n", gruppo.Entry[z].UUID)
		var informazione data
		for w := 0; w < len(gruppo.Entry[z].SingleEntries); w++ {
			v := gruppo.Entry[z].SingleEntries[w]
			if v.Key == "Password" {
				informazione.Password = v.Value
			}
			if v.Key == "UserName" {
				informazione.Username = v.Value
			}
			if v.Key == "Title" {
				informazione.Account = v.Value
			}
		}
		informazioni = append(informazioni, informazione)
	}
	for z := 0; z < len(gruppo.Group); z++ {
		informazioni = append(informazioni, gruppo.Group[z].parseGroup()...)
	}
	return informazioni
}

func main() {
	var database result
	var informazioni []data
	//	testInfo := []data{{"test", "test", "password"}}

	// Open our xmlFile
	xmlFile, err := os.Open("Database.xml")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Successfully Opened Database.xml")

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
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()
	err = xml.Unmarshal(data, &database)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	//fmt.Printf("XMLName: %#v\n", database.XMLName)
	for i := 0; i < len(database.Root); i++ {
		//fmt.Printf("group: %v\n", database.Root[i])
		for j := 0; j < len(database.Root[i].Group); j++ {
			informazioni = append(informazioni, database.Root[i].Group[j].parseGroup()...)
		}
	}

	//	informazioni = testInfo

	for i := 0; i < len(informazioni); i++ {
		v := informazioni[i]
		//fmt.Printf("[%s] : %s = %s \n", v.Account, v.Username, v.Password)

		data := []byte(v.Password)
		hash := fmt.Sprintf("%X", sha1.Sum(data))
		short := hash[:5]
		long := hash[5:]

		res, err := http.Get("https://api.pwnedpasswords.com/range/" + short)
		if err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
		bodyStr := string(body[:len(body)])
		entries := strings.Split(bodyStr, "\r\n")

		for l := 0; l < len(entries); l++ {
			spl := strings.Split(entries[l], ":")
			pwd := spl[0]
			num := spl[1]
			if long == pwd {
				fmt.Printf("CAMBIARE PWD PER [ %s ] : %s risultati\n", v.Account, num)
			}
		}

		time.Sleep(2000 * time.Millisecond)
	}
}
