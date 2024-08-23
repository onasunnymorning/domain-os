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
	"slices"
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
	CHUNKSIZE     = 1000
	API_HOST      = "192.168.64.6"
	API_PORT      = "8080"
	ERR_DUPL_PK   = "ERROR: duplicate key value violates unique constraint \"registrars_pkey\" (SQLSTATE 23505)"
	ERR_DUPL_NAME = "ERROR: duplicate key value violates unique constraint \"registrars_name_key\" (SQLSTATE 23505)"
)

var (
	URL              = "http://" + API_HOST + ":" + API_PORT + "/registrars"
	DUPLICATE_ERRORS = []string{
		ERR_DUPL_PK,
		ERR_DUPL_NAME,
	}
)

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
	// Retrieve all IANAResgistrars from the API
	ianaRegistrars, err := GetIANARegistrars()
	if err != nil {
		log.Fatalf("[ERR] error getting IANA registrars: %v", err)
	}
	countIANARars := len(ianaRegistrars)
	log.Printf("[INFO] got %d IANA registrars\n", countIANARars)

	// Read in the CSV file from ICANN containing richer contact details for the registrars
	csvRars, err := getCSVRegistrarsFromFile(*filename)
	if err != nil {
		log.Fatalf("[ERR] error getting CSV registrars from file: %v", err)
	}
	countCSVRars := len(csvRars)
	log.Printf("[INFO] %d registrars found in CSV file\n", countCSVRars)

	cmds, err := getCreateCommands(csvRars, ianaRegistrars)
	if err != nil {
		log.Fatalf("[ERR] error getting create commands: %v", err)
	}
	countCmds := len(cmds)
	log.Printf("[INFO] %d create commands created\n", countCmds)

	// err = bulkCreateRegistrarsThroughAPI(countCmds, CHUNKSIZE, cmds)
	// if err != nil {
	// 	log.Fatalf("[ERR] error creating registrars: %v", err)
	// }

	err = createRegistrars(cmds)
	if err != nil {
		log.Fatalf("[ERR] error creating registrars: %v", err)
	}

	err = updateStatus(ianaRegistrars)
	if err != nil {
		log.Fatalf("[ERR] error updating registrar statuses: %v", err)
	}

	// Collate the infromation from both ICANN and IANA sources into create commands

	// createCommands, err := getCreateRegistrarCommandsFromFile(*filename)
	// if err != nil {
	// 	log.Fatalf("[ERR] error getting create commands from file: %v", err)
	// }

	// err = createRegistrars(createCommands)
	// if err != nil {
	// 	log.Fatalf("[ERR] error creating registrars: %v", err)
	// }

	// err = updateRegistrarStatuses(createCommands)
	// if err != nil {
	// 	log.Fatalf("[ERR] error updating registrar statuses: %v", err)
	// }

	// // Create CreateRegistrarCommands for the terminated IANARegistrars
	// fmt.Println("[INFO] Createing terminated registrars")
	// terminatedCreateCommands, err := getCreateCommandsForTerminatedRegistrars(ianaRegistrars)
	// if err != nil {
	// 	log.Fatalf("[ERR] error getting create commands for terminated registrars: %v", err)
	// }

	// err = createRegistrars(terminatedCreateCommands)
	// if err != nil {
	// 	log.Fatalf("[ERR] error creating terminated registrars: %v", err)
	// }

	// err = updateRegistrarStatuses(terminatedCreateCommands)
	// if err != nil {
	// 	log.Fatalf("[ERR] error updating terminated registrar statuses: %v", err)
	// }

}

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

// APIError represents an error returned by the API
type APIError struct {
	Error string `json:"error"`
}

// SyncIANARegistrars triggers the API backend to refresh the ICANN registrars
func SyncIANARegistrars() {
	URL := "http://" + API_HOST + ":" + API_PORT + "/sync/iana-registrars"
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
func GetIANARegistrars() ([]entities.IANARegistrar, error) {
	var returnVal []entities.IANARegistrar
	// First get a count of the objects we are pulling
	count, err := getCount()
	if err != nil {
		return returnVal, fmt.Errorf("could not get count of IANA registrars: %v", err)
	}
	log.Printf("[INFO] getting %d IANA registrars\n", count)
	// Set the URL
	URL := "http://" + API_HOST + ":" + API_PORT + "/ianaregistrars?pagesize=1000"
	// Get the first batch
	result, err := getBatch(URL)
	if err != nil {
		return returnVal, fmt.Errorf("error getting IANA registrars via API(%s): %v", URL, err)
	}
	// Check if result.Data is a pointer to a slice
	registrars, ok := result.Data.(*[]entities.IANARegistrar)
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
		registrars, ok := result.Data.(*[]entities.IANARegistrar)
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
	URL := "http://" + API_HOST + ":" + API_PORT + "/ianaregistrars/count"
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
	registrars := []entities.IANARegistrar{}
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
	URL := "http://" + API_HOST + ":" + API_PORT + "/ianaregistrars/" + strconv.Itoa(ianaID)
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

// getCreateRegistrarCommandsFromFile returns a slice of commands to create the registrars
func getCreateRegistrarCommandsFromFile(filename string) ([]commands.CreateRegistrarCommand, error) {

	// Get the CSVRegistrars from the file
	registrars, err := getCSVRegistrarsFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error getting CSVRegistrars from file: %v", err)
	}

	// Convert the CSVRegistrars to CreateRegistrarCommands
	createCommands, err := convertCSVRegistrarsToCommands(registrars)
	if err != nil {
		return nil, fmt.Errorf("error converting CSVRegistrars to CreateRegistrarCommands: %v", err)
	}

	return createCommands, nil
}

// getCSVRegistrarsFromFile reads the CSV file and returns a slice of CSVRegistrars
func getCSVRegistrarsFromFile(filename string) ([]CSVRegistrar, error) {

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	log.Printf("[INFO] preparing data in file %s\n", filename)

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
			return nil, fmt.Errorf("error converting IANAID to int: %v", err)
		}

		registrars[i-1] = CSVRegistrar{
			Name:    line[0],
			IANAID:  ianaID,
			Country: line[2],
			Contact: line[3],
			Link:    line[4],
		}
	}

	return registrars, nil
}

// convertCSVRegistrarsToCommands converts a slice of CSVRegistrars to a slice of CreateRegistrarCommands
func convertCSVRegistrarsToCommands(registrars []CSVRegistrar) ([]commands.CreateRegistrarCommand, error) {

	// Covert to a slice of CreateRegistrarCommands
	createCommands := make([]commands.CreateRegistrarCommand, len(registrars))
	seenClid := make(map[string]bool)
	seenName := make(map[string]bool)
	for i, r := range registrars {
		addr, err := r.Address()
		if err != nil {
			return nil, fmt.Errorf("error getting address for registrar %s: %v", r.Name, err)
		}

		clidName, err := r.CreateSlug()
		if err != nil {
			return nil, fmt.Errorf("error creating slug for registrar %s: %v", r.Name, err)
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
		if seenClid[rarCmd.ClID] {
			return nil, fmt.Errorf("duplicate Registrar.ClID: %s", rarCmd.ClID)
		}
		seenClid[rarCmd.ClID] = true

		// Check duplicate Name
		if seenName[rarCmd.Name] {
			rarCmd.Name = rarCmd.Name + "-2"
		}
		seenName[rarCmd.ClID] = true

		// Add the command to the slice
		createCommands[i] = rarCmd
	}
	return createCommands, nil
}

// createRegistrars creates the registrars in the database from a slice of CreateRegistrarCommands
func createRegistrars(createCommands []commands.CreateRegistrarCommand) error {

	// Create the registrars
	bar := progressbar.Default(int64(len(createCommands)), "Creating Registrars")
	for _, cmd := range createCommands {
		err := createRegistrar(cmd)
		if err != nil {
			if err.Error() == ERR_DUPL_NAME {
				// If there is a name collision, try and rename the registrar
				cmd.Name = cmd.Name + "-2"
				err := createRegistrar(cmd)
				if err == nil {
					// If this resolved it, continue
					continue
				}
			}
			log.Fatalf("[ERR] error creating registrar %s: %v", cmd.Name, err)
		}
		bar.Add(1)
	}
	return nil
}

// updateRegistrarStatuses updates the status of the registrars in the database based on the IANARegistrars
func updateRegistrarStatuses(createCommands []commands.CreateRegistrarCommand) error {

	// Update the status of the newly creaed registrars to match the IANARegistrars' Status
	bar := progressbar.Default(int64(len(createCommands)), "Setting Registrar Status")
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
		URL := "http://" + API_HOST + ":" + API_PORT + "/registrars/" + r.ClID + "/status/" + status
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
	return nil
}

// getCreateCommandsForTerminatedRegistrars returns a slice of CreateRegistrarCommands for the terminated registrars
func getCreateCommandsForTerminatedRegistrars(irars []entities.IANARegistrar) ([]commands.CreateRegistrarCommand, error) {
	var createCommands []commands.CreateRegistrarCommand
	// dummy postalinfo
	a, err := entities.NewAddress("Vichayitos", "PE")
	if err != nil {
		return nil, fmt.Errorf("error creating address: %v", err)
	}
	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, a)
	if err != nil {
		return nil, fmt.Errorf("error creating postalinfo: %v", err)
	}
	postalInfo := [2]*entities.RegistrarPostalInfo{
		pi,
	}
	// loop over the IANARegistrars and find the terminated ones, create a CreateRegistrarCommand for these
	for _, irar := range irars {
		if irar.Status != "Terminated" {
			continue
		}
		// Create a slug
		csv := CSVRegistrar{
			IANAID: irar.GurID,
			Name:   irar.Name,
		}
		slug, err := csv.CreateSlug()
		if err != nil {
			return nil, fmt.Errorf("error creating slug for registrar %s: %v", irar.Name, err)
		}
		// Create a CreateRegistrarCommand
		cmd := commands.CreateRegistrarCommand{
			ClID:       slug,
			Name:       irar.Name,
			Email:      "i.need@2be.replaced",
			GurID:      irar.GurID,
			URL:        irar.RdapURL,
			PostalInfo: postalInfo,
		}
		// Add the command to the slice
		createCommands = append(createCommands, cmd)
	}
	return createCommands, nil
}

// getCreateCommands takes a slice of CSVRegistrars and a slice of IANARegistrars and returns a slice of CreateRegistrarCommands
func getCreateCommands(csvRegistrars []CSVRegistrar, icannRegistrars []entities.IANARegistrar) ([]commands.CreateRegistrarCommand, error) {
	skipped := []string{}
	seen := make(map[string]bool)
	var createCommands []commands.CreateRegistrarCommand

	// Create a dummy postalinfo
	a, err := entities.NewAddress("Vichayitos", "PE")
	if err != nil {
		return nil, fmt.Errorf("error creating address: %v", err)
	}
	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, a)
	if err != nil {
		return nil, fmt.Errorf("error creating postalinfo: %v", err)
	}

	// Create a map of IANARegistrars for easy lookup by IANAID
	ianaMap := make(map[int]entities.IANARegistrar)
	for _, irar := range icannRegistrars {
		ianaMap[irar.GurID] = irar
	}

	// Create a map of CSVRegistrars for easy lookup by IANAID
	csvMap := make(map[int]CSVRegistrar)
	for _, crar := range csvRegistrars {
		csvMap[crar.IANAID] = crar
	}

	// Loop over the IANARegistrars and create a CreateRegistrarCommand for each, enriched with the contact information from the CSVRegistrars
	for _, irar := range icannRegistrars {

		// Omit the reserved registrars
		if irar.Status == entities.IANARegistrarStatusReserved {
			log.Printf("[WARN] Registrar %s with GurID %d is reserved, skipping\n", irar.Name, irar.GurID)
			skipped = append(skipped, strconv.Itoa(irar.GurID)+" - "+irar.Name)
			continue
		}

		clid, err := irar.CreateClID()
		if err != nil {
			return nil, fmt.Errorf("error creating ClID for registrar %d - %s: %v", irar.GurID, irar.Name, err)
		}

		if seen[irar.Name] {
			irar.Name = irar.Name + "-2"
		}
		seen[irar.Name] = true

		// Create the command with dummy information
		cmd := commands.CreateRegistrarCommand{
			ClID:  clid.String(),
			GurID: irar.GurID,
			Name:  irar.Name,
			Email: "i.need@2be.replaced",
			PostalInfo: [2]*entities.RegistrarPostalInfo{
				pi,
			},
		}

		// try and enrich the command with the contact information from the CSVRegistrars - only if it exists
		csv, ok := csvMap[irar.GurID]
		if ok {
			a, err := csv.Address()
			if err != nil {
				return nil, fmt.Errorf("error getting address for registrar %s: %v", csv.Name, err)
			}
			// if the Address is ASCII add an int postalinfo, else add a loc postalinfo
			if isacii, _ := a.IsASCII(); isacii {
				cmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
					Type:    entities.PostalInfoEnumTypeINT,
					Address: a,
				}
			} else {
				cmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
					Type:    entities.PostalInfoEnumTypeLOC,
					Address: a,
				}
			}

			cmd.Email = csv.ContactEmail()
			cmd.Voice = csv.ContactPhone()
			cmd.URL = csv.Link

		}

		// Add the command to the slice
		createCommands = append(createCommands, cmd)

	}

	return createCommands, nil
}

// updateStatus updates the status of the registrars in the DB based on the IANARegistrars' status
func updateStatus(irars []entities.IANARegistrar) error {
	// Create a map of IANARegistrars for easy lookup by IANAID
	ianaMap := make(map[int]entities.IANARegistrar)
	for _, irar := range irars {
		ianaMap[irar.GurID] = irar
	}

	// Loop over the IANARegistrars and update the status of the registrars in the DB using an API call
	bar := progressbar.Default(int64(len(irars)), "Updating Registrar Status")
	for _, irar := range irars {
		status := irar.Status
		// Skip if we're dealing with a reserved registrar
		if status == entities.IANARegistrarStatusReserved {
			continue
		}
		// Map 'Accredited' status in IANA/ICANN source to 'ok' for the API and RDE RFC
		if status == entities.IANARegistrarStatusAccredited {
			status = "ok"
		}
		clid, err := irar.CreateClID()
		if err != nil {
			return fmt.Errorf("error creating ClID for registrar %d - %s: %v", irar.GurID, irar.Name, err)
		}

		URL := "http://" + API_HOST + ":" + API_PORT + "/registrars/" + clid.String() + "/status/" + string(status)
		req, err := http.NewRequest(http.MethodPut, URL, nil)
		if err != nil {
			return fmt.Errorf("error creating PUT request to update registrar status: %v", err)
		}
		// Create a new HTTP client
		client := &http.Client{}
		// Send the PUT request
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error updating registrar status via API(%s): %v", URL, err)
		}

		bar.Add(1)
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				log.Printf("[INFO] Registrar %s with GurID %d not found, skipping\n", irar.Name, irar.GurID)
				continue
			}
			return fmt.Errorf("error updating registrar status: %v - %v", resp.Status, URL)
		}

	}
	return nil
}

// createRegistrar creates a single registrar in the database through an API call
func createRegistrar(cmd commands.CreateRegistrarCommand) error {
	postBody, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("error marshaling command: %v", err)
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
		return fmt.Errorf("error reading response body: %v", err)
	}

	// Mashall into a Registrar
	var registrar entities.Registrar
	err = json.Unmarshal(body, &registrar)
	if err != nil {
		return fmt.Errorf("error unmarshaling response for %s: %v", cmd.Name, err)
	}

	if resp.StatusCode != http.StatusCreated {
		// unmarshall the body
		var apiErr APIError
		err = json.Unmarshal(body, &apiErr)
		if err != nil {
			return fmt.Errorf("error unmarshaling error response for %s: %v", cmd.Name, err)
		}
		if slices.Contains(DUPLICATE_ERRORS, apiErr.Error) {
			if apiErr.Error == ERR_DUPL_PK {
				// the registrar exists, continue
				return nil
			}
			if apiErr.Error == ERR_DUPL_NAME {
				// Sometimes the name is the same but the IANAID is different
				// log.Printf("[WARN] Registrar %s already exists, continueing\n", cmd.Name)
				// fmt.Println(cmd)
				// fmt.Println(apiErr)
				// log.Printf("Try renaming the registrar %s to %s and try again\n", cmd.Name, cmd.Name+"2")
				return errors.New(ERR_DUPL_NAME)
			}
		}
		return fmt.Errorf("error creating registrar %s: %v - %v", cmd.Name, resp.Status, string(body))
	}
	return nil
}

// bulkCreateRegistrarsAPI creates registrars in BULK throug one API command
func bulkCreateRegistrarsThroughAPI(total, chunkSize int, cmds []commands.CreateRegistrarCommand) error {
	for i := 0; i < total; i += chunkSize {
		// Determine the end of the current chunk
		end := i + chunkSize
		if end > total {
			end = total
		}

		// Slice the commands to create a chunk
		chunk := cmds[i:end]

		URL := "http://" + API_HOST + ":" + API_PORT + "/registrars-bulk"
		postBody, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("error marshaling command: %v", err)
		}

		resp, err := http.Post(URL, "application/json", bytes.NewBuffer(postBody))
		if err != nil {
			return fmt.Errorf("error sending create command to API: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("error creating registrars in bulk through API: %s", string(body))
		}
	}

	return nil
}
