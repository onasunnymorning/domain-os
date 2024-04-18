package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/biter777/countries"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// This script is intended to import the ICANN 2013 Registrar List into the database.
// Use this when initializing the database for the first time.
// The file can be downloaded from the ICANN website at: https://www.icann.org/en/accredited-registrars

// CSVRegistrar represents a registrar in the CSV file from ICANN
// Header: "Registrar Name","IANA Number","Country/Territory","Public Contact","Link"
type CSVRegistrar struct {
	Name    string
	IANAID  int
	Country string
	Contact string
	Link    string
}

// ContactName returns the name of the contact person (the first part of the contact string, before the `+` sign)
func (r CSVRegistrar) ContactName() string {
	if !strings.Contains(r.Contact, "+") {
		return strings.Split(r.Contact, "null")[0]
	}
	return strings.Split(r.Contact, "+")[0]
}

// CreateSlug creates a slug from the registrar name that is a valid ClIDType
func (r CSVRegistrar) CreateSlug() (string, error) {
	// split the string by comma ',' and return the frist part
	slug := strings.Split(r.Name, ",")[0]
	// lowercase the string
	slug = strings.ToLower(slug)
	// Remove all Non-ASCII characters
	slug = entities.RemoveNonASCII(slug)
	// replace all spaces ' ' with dashes '-'
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove all Non-AlphaNumeric characters
	slug = entities.RemoveNonAlphaNumeric(slug)
	// remove all dots '.'
	slug = strings.ReplaceAll(slug, ".", "")
	// if the string is longer than 16 characters, truncate it to 13 to leave some space for the IANAID
	if len(slug) > 10 {
		slug = slug[:10]
	}
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// prepend the IANAID to the slug
	slug = fmt.Sprintf("%d-%s", r.IANAID, slug)
	fmt.Println(slug)
	// validate as a ClIDType
	clidSlug, err := entities.NewClIDType(slug)
	return clidSlug.String(), err
}

// ContactPhone returns the phone number of the contact person (the second part of the contact string, after the `+` sign)
func (r CSVRegistrar) ContactPhone() string {
	var phoneSlice []string
	if !strings.Contains(r.Contact, "+") {
		// in case there is no phone number, the phone number will be `null`
		// phoneSlice = strings.Split(strings.Split(r.Contact, "null")[1], " ")[0 : len(strings.Split(strings.Split(r.Contact, "null")[1], " "))-1]
		phoneSlice = []string{""}
	} else {
		phoneSlice = strings.Split(strings.Split(r.Contact, "+")[1], " ")[0 : len(strings.Split(strings.Split(r.Contact, "+")[1], " "))-1]
	}

	// join the phoneSlice to get the phone number
	cleaned := cleanPhone([]byte(strings.Join(phoneSlice, " ")))
	// replace the first space with a '.'
	cleaned = strings.Replace(cleaned, " ", ".", 1)
	// remove all remaining spaces
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Validate the phone number
	validated, err := entities.NewE164Type("+" + cleaned)
	if err != nil {
		// log.Printf("Error validating phone number %s: %v - Removing phone number", cleaned, err)
		return ""
	}

	return validated.String()

}

// cleanPhone removes all characters from the phone number string that are not numbers
func cleanPhone(s []byte) string {
	j := 0
	for _, b := range s {
		if ('0' <= b && b <= '9') || b == ' ' {
			s[j] = b
			j++
		}
	}
	return string(s[:j])
}

// ContactEmail returns the URL of the contact person (the last part of the contact string)
func (r CSVRegistrar) ContactEmail() string {
	return strings.Split(r.Contact, " ")[len(strings.Split(r.Contact, " "))-1]
}

// CountryCode returns the country code of the registrar
func (r CSVRegistrar) Address() (*entities.Address, error) {
	country := countries.ByName(r.Country)
	// There are some exceptions in the file
	if strings.Contains(r.Country, "United Kingdom") {
		country = countries.ByName("United Kingdom")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Hong Kong") {
		country = countries.ByName("Hong Kong")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Marshall Islands") {
		country = countries.ByName("Marshall Islands")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Panama") {
		country = countries.ByName("Panama")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Taipei") {
		country = countries.ByName("Taiwan")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	// 2024-04-13 - The following entry contains an empty country field so adding a manual check for the IANAID 3874:
	// "Butterfly Asset Management Pte. Ltd",3874,,"Jianwen Chen +65 83516253 birichcom@163.com","http://birich.com"
	if r.IANAID == 3874 {
		country = countries.ByName("Singapore")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}

	if !country.IsValid() {
		return nil, fmt.Errorf("country not found: %s", r.Country)
	}

	return &entities.Address{
		City:        entities.PostalLineType(country.Capital().Info().Name),
		CountryCode: entities.CCType(country.Alpha2()),
	}, nil
}

func main() {
	// FLAGS
	filename := flag.String("f", "", "(path to) filename")
	flag.Parse()

	if *filename == "" {
		log.Fatal("Please provide a filename")
	}

	// Open the file
	file, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true // To avoid `parse error on line 1, column 4: bare " in non-quoted-field` error
	data, err := reader.ReadAll()
	if err != nil {
		log.Fatalln(err)
	}

	// Make a slice of CSVRegistrars
	registrars := make([]CSVRegistrar, len(data)-1)
	for i, line := range data {
		if i == 0 {
			// Skip the header
			continue
		}

		// convert the IANAID to an int
		ianaID, err := strconv.Atoi(line[1])
		if err != nil {
			log.Fatalf("Error converting IANAID to int: %v", err)
		}

		registrars[i-1] = CSVRegistrar{
			Name:    line[0],
			IANAID:  ianaID,
			Country: line[2],
			Contact: line[3],
			Link:    line[4],
		}
	}

	// Covert to a slice of CreateRegistrarCommands
	createCommands := make([]commands.CreateRegistrarCommand, len(registrars))
	seen := make(map[string]bool)
	for i, r := range registrars {
		addr, err := r.Address()
		if err != nil {
			log.Fatalf("Error getting address: %v", err)
		}

		clidName, err := r.CreateSlug()
		if err != nil {
			log.Printf("Error creating slug for registrar %s: %v", r.Name, err)
		}
		rarCmd := commands.CreateRegistrarCommand{
			ClID:  clidName,
			Name:  r.Name,
			Email: r.ContactEmail(),
			Voice: r.ContactPhone(),
			GurID: r.IANAID,
			URL:   r.Link,
			PostalInfo: [2]*entities.RegistrarPostalInfo{
				{
					Type:    entities.PostalInfoEnumTypeINT,
					Address: addr,
				},
			},
		}

		// Check for duplicate ClIDs
		if seen[rarCmd.ClID] {
			log.Fatalf("Duplicate Registrar.ClID: %s", rarCmd.ClID)
		}
		seen[rarCmd.ClID] = true

		createCommands[i] = rarCmd
	}

	// Create the registrars
	for _, cmd := range createCommands {
		postBody, err := json.Marshal(cmd)
		if err != nil {
			log.Fatalf("Error marshaling command: %v", err)
		}

		resp, err := http.Post("http://localhost:8080/registrars", "application/json", bytes.NewBuffer(postBody))
		if err != nil {
			log.Fatalf("Error posting registrar %s: %v", cmd.Name, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}

		if resp.StatusCode != http.StatusCreated {
			log.Fatalf("Error creating registrar %s: %v - %v", cmd.Name, resp.Status, string(body))
		}

		log.Printf("Registrar %s created as %s\n", cmd.Name, cmd.ClID)
	}
}
