package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
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
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/schollz/progressbar/v3"
)

// This script is intended to import the ICANN 2013 Registrar List into the database.
// Use this when initializing the database for the first time.
// The file can be downloaded from the ICANN website at: https://www.icann.org/en/accredited	-registrars

const (
	API_HOST = "localhost"
	API_POST = "8080"

	EXISTS_ERRMSG = "ERROR: duplicate key value violates unique constraint \"registrars_pkey\" (SQLSTATE 23505)"
)

var (
	URL = "http://" + API_HOST + ":" + API_POST + "/registrars"
)

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
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// prepend the IANAID to the slug
	slug = fmt.Sprintf("%d-%s", r.IANAID, slug)
	// if the string is longer than 16 characters, truncate it
	if len(slug) > 16 {
		slug = slug[:16]
	}
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
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
	cleaned := cleanPhoneNumber([]byte(strings.Join(phoneSlice, " ")))
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

// cleanPhoneNumber removes all characters from the phone number string that are not numbers
func cleanPhoneNumber(s []byte) string {
	j := 0
	for _, b := range s {
		if ('0' <= b && b <= '9') || b == ' ' {
			s[j] = b
			j++
		}
	}
	return string(s[:j])
}

// ContactEmail returns the email of the contact person (the last part of the contact string)
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
	sync := flag.Bool("s", false, "sync IANA registrars")
	flag.Parse()

	if *filename == "" {
		log.Fatal("[ERR] please provide a filename")
	}

	// If requested sync the IANARegistrars on the backend first
	if *sync {
		log.Println("[INFO] syncing IANA registrars")
		SyncIANARegistrars()
	}
	// Retrieve all IANAResgistrars from the API so we can SET THE CORRECT STATUS
	ianaRegistrars, err := GetIANARegistrars()
	if err != nil {
		log.Fatalf("[ERR] error getting IANA registrars: %v", err)
	}
	log.Printf("[INFO] got %d IANA registrars\n", len(ianaRegistrars))

	// Open the file
	file, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	log.Printf("[INFO] preparing data in file %s\n", *filename)

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
			log.Fatalf("[ERR] error converting IANAID to int: %v", err)
		}

		registrars[i-1] = CSVRegistrar{
			Name:    line[0],
			IANAID:  ianaID,
			Country: line[2],
			Contact: line[3],
			Link:    line[4],
		}
	}
	log.Printf("[INFO] %d registrars found\n", len(registrars))

	// Covert to a slice of CreateRegistrarCommands
	createCommands := make([]commands.CreateRegistrarCommand, len(registrars))
	seen := make(map[string]bool)
	for i, r := range registrars {
		addr, err := r.Address()
		if err != nil {
			log.Fatalf("[ERR] error getting address for registrar %s : %v", r.Name, err)
		}

		clidName, err := r.CreateSlug()
		if err != nil {
			log.Printf("[ERR] error creating slug for registrar %s: %v", r.Name, err)
		}
		rarCmd := commands.CreateRegistrarCommand{
			ClID:       clidName,
			Name:       r.Name,
			Email:      r.ContactEmail(),
			Voice:      r.ContactPhone(),
			GurID:      r.IANAID,
			URL:        r.Link,
			PostalInfo: [2]*entities.RegistrarPostalInfo{},
		}

		// if the Address is ASCII add an int postalinfo, else add a loc postalinfo
		if isacii, _ := addr.IsASCII(); isacii {
			rarCmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
				Type:    entities.PostalInfoEnumTypeINT,
				Address: addr,
			}
		} else {
			rarCmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
				Type:    entities.PostalInfoEnumTypeLOC,
				Address: addr,
			}
		}

		// Check for duplicate ClIDs
		if seen[rarCmd.ClID] {
			log.Fatalf("[ERR] duplicate Registrar.ClID: %s", rarCmd.ClID)
		}
		seen[rarCmd.ClID] = true

		// Add the command to the slice
		createCommands[i] = rarCmd
	}

	// Create the registrars
	bar := progressbar.Default(int64(len(createCommands)), "Creating Registrars")
	for _, cmd := range createCommands {
		postBody, err := json.Marshal(cmd)
		if err != nil {
			log.Fatalf("[ERR] error marshaling command: %v", err)
		}

		resp, err := http.Post(URL, "application/json", bytes.NewBuffer(postBody))
		if err != nil {
			log.Println(cmd)
			log.Println(URL)
			log.Fatalf("[ERR] error send create command to API %s: %v", cmd.Name, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}

		// Mashall into a Registrar
		var registrar entities.Registrar
		err = json.Unmarshal(body, &registrar)
		if err != nil {
			log.Fatalf("[ERR] error unmarshaling response for %s: %v", cmd.Name, err)
		}

		bar.Add(1)
		if resp.StatusCode != http.StatusCreated {
			// unmarshall the body
			var apiErr APIError
			err = json.Unmarshal(body, &apiErr)
			if err != nil {
				log.Fatalf("[ERR]error unmarshaling error response for %s: %v", cmd.Name, err)
			}
			if apiErr.Error == EXISTS_ERRMSG {
				// log.Printf("[WARN] Registrar %s already exists, continueing\n", cmd.Name)
				continue
			}
			log.Fatalf("[ERR] error creating registrar %s: %v - %v", cmd.Name, resp.Status, string(body))
		}

		// log.Printf("Registrar %s created as %s\n", cmd.Name, cmd.ClID)
	}

	// Update the status of the newly creaed registrars to match the IANARegistrars' Status
	bar = progressbar.Default(int64(len(createCommands)), "Setting Registrar Status")
	for _, r := range createCommands {
		status, err := getIANARegistrarStatus(r.GurID)
		if err != nil {
			log.Fatalf("[ERR] error getting IANA registrar status for %s: %v", r.Name, err)
		}
		// Map Accredited => ok
		if status == "Accredited" {
			status = "ok"
		}

		// Update that registrar's status
		URL := "http://" + API_HOST + ":" + API_POST + "/registrars/" + r.ClID + "/status/" + status
		req, err := http.NewRequest(http.MethodPut, URL, nil)
		if err != nil {
			log.Fatalf("[ERR] error creating PUT request to update registrar status: %v", err)
		}
		// Create a new HTTP client
		client := &http.Client{}
		// Send the PUT request
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("[ERR] error updating registrar status via API(%s): %v", URL, err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("[ERR] error updating registrar status: %v - %v", resp.Status, URL)
		}

		bar.Add(1)

	}
}

// APIError represents an error returned by the API
type APIError struct {
	Error string `json:"error"`
}

// SyncIANARegistrars triggers the API backend to refresh the ICANN registrars
func SyncIANARegistrars() {
	URL := "http://" + API_HOST + ":" + API_POST + "/sync/iana-registrars"
	req, err := http.NewRequest(http.MethodPut, URL, nil)
	if err != nil {
		log.Fatalf("[ERR] error creating PUT request to sync IANA registrars: %v", err)
	}
	// Create a new HTTP client
	client := &http.Client{}

	// Send the PUT request
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if err != nil {
		log.Fatalf("[ERR] error syncing IANA regsitrars via API(%s): %v", URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("[ERR] error syncing IANA registrars: %v - %v", resp.Status, string(body))
	}
	log.Println("[INFO] IANA registrars updated")
}

// GetIANARegsitrars queries the API for all IANA registrars
func GetIANARegistrars() ([]entities.Registrar, error) {
	var returnVal []entities.Registrar
	// First get a count of the objects we are pulling
	count, err := getCount()
	if err != nil {
		return returnVal, fmt.Errorf("could not get count of IANA registrars: %v", err)
	}
	log.Printf("[INFO] getting %d IANA registrars\n", count)
	// Set the URL
	URL := "http://" + API_HOST + ":" + API_POST + "/ianaregistrars?pagesize=1000"
	// Get the first batch
	result, err := getBatch(URL)
	if err != nil {
		return returnVal, fmt.Errorf("error getting IANA registrars via API(%s): %v", URL, err)
	}
	// Check if result.Data is a pointer to a slice
	registrars, ok := result.Data.(*[]entities.Registrar)
	if !ok {
		return returnVal, fmt.Errorf("unexpected type for result.Data: %T", result.Data)
	}
	// Append the first batch to the returnVal
	returnVal = append(returnVal, *registrars...)
	// Get the rest of the batches
	for result.Meta.NextLink != "" {
		result, err = getBatch(result.Meta.NextLink)
		if err != nil {
			return returnVal, fmt.Errorf("error getting IANA registrars via API(%s): %v", result.Meta.NextLink, err)
		}
		// Check if result.Data is a pointer to a slice
		registrars, ok := result.Data.(*[]entities.Registrar)
		if !ok {
			return returnVal, fmt.Errorf("unexpected type for result.Data: %T", result.Data)
		}
		// Append the first batch to the returnVal
		returnVal = append(returnVal, *registrars...)
	}

	return returnVal, nil
}

// getCount returns the count of the IANARegistrar objects we are pulling
func getCount() (int64, error) {
	URL := "http://" + API_HOST + ":" + API_POST + "/ianaregistrars/count"
	var countResult response.CountResult
	// Make the request
	resp, err := http.Get(URL)
	if err != nil {
		return int64(0), errors.Join(fmt.Errorf("could not build count url(%s)", URL), err)
	}
	// Check the response status
	if resp.StatusCode != http.StatusOK {
		fmt.Println(URL)
		return int64(0), errors.Join(fmt.Errorf("could not get count of IANA registrars: %v", resp.Status), err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return int64(0), errors.Join(fmt.Errorf("could not read response body"), err)
	}
	err = json.Unmarshal(body, &countResult)
	if err != nil {
		fmt.Println(string(body))
		return int64(0), errors.Join(fmt.Errorf("could not unmarshal response body"), err)
	}
	return countResult.Count, nil
}

// getBatch returns a batch of IANARegistrars
func getBatch(url string) (*response.ListItemResult, error) {
	registrars := []entities.Registrar{}
	listResult := response.ListItemResult{}
	listResult.Data = &registrars

	// Make the request
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error getting IANA regsitrars via API(%s)", URL), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting IANA registrars: %v - %v", resp.Status, string(body))
	}
	// Unmarshal the result
	err = json.Unmarshal(body, &listResult)
	if err != nil {
		return nil, errors.Join(errors.New("error unmarshaling response from API"), err)
	}

	return &listResult, nil

}

// getIANARegsitrarStatus returns the status of the IANARegistrar with the given IANAID
func getIANARegistrarStatus(ianaID int) (string, error) {
	URL := "http://" + API_HOST + ":" + API_POST + "/ianaregistrars/" + strconv.Itoa(ianaID)
	var irar entities.IANARegistrar
	// Make the request
	resp, err := http.Get(URL)
	if err != nil {
		return "", errors.Join(fmt.Errorf("error getting IANA registrar via API(%s)", URL), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting IANA registrar: %v - %v", resp.Status, string(body))
	}
	// Unmarshal the result
	err = json.Unmarshal(body, &irar)
	if err != nil {
		return "", errors.Join(errors.New("error unmarshaling response from API"), err)
	}

	return string(irar.Status), nil
}
