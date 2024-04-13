package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/schollz/progressbar/v3"
)

var (
	ErrDecodingToken = errors.New("error decoding token")
	ErrNoDepositTag  = errors.New("no deposit tag found")
	ErrNoHeaderTag   = errors.New("no header tag found")
)

// XMLEscrowAnalysisService implements XMLEscrowAnalysisService interface
type XMLEscrowAnalysisService struct {
	Deposit           entities.RDEDeposit             `json:"deposit"`
	Header            entities.RDEHeader              `json:"header"`
	Registrars        []entities.RDERegistrar         `json:"registrars"`
	IDNs              []entities.RDEIdnTableReference `json:"idns"`
	RegsistrarMapping entities.RegsitrarMapping       `json:"registrarMapping"`
	Analysis          entities.EscrowAnalysis         `json:"analysis"`
}

// NewXMLEscrowService creates a new instance of EscrowService
func NewXMLEscrowService(XMLFilename string) (*XMLEscrowAnalysisService, error) {
	// Fail fast if we can't open the file
	f, _ := os.Open(XMLFilename)
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	d := XMLEscrowAnalysisService{}
	// Set filename and size
	d.Deposit.FileName = XMLFilename
	d.Deposit.FileSize = fi.Size()
	log.Printf("Escow file %s is %d MB\n", XMLFilename, d.Deposit.FileSize/1024/1024)

	// Initialize the registrar mapping
	d.RegsistrarMapping = entities.NewRegistrarMapping()

	return &d, nil
}

// GetDeposit returns the RdeDeposit element in JSON format
func (svc *XMLEscrowAnalysisService) GetDepositJSON() string {
	jsonDepositBytes, err := json.MarshalIndent(svc.Deposit, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonDepositBytes)
}

// GetHeader returns the RdeHeader element in JSON format
func (svc *XMLEscrowAnalysisService) GetHeaderJSON() string {
	jsonHeaderBytes, err := json.MarshalIndent(svc.Header, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonHeaderBytes)
}

// Analyzes the deposit XML tag
func (svc *XMLEscrowAnalysisService) AnalyzeDepostTag() error {
	// our found flag
	found := false

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Looking for deposit tag in %s ... \n", svc.Deposit.FileName)
	for {
		if found {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type deposit
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "deposit" {
				if err := d.DecodeElement(&svc.Deposit, &se); err != nil {
					return fmt.Errorf("error decoding deposit: %s", tokenErr)
				}
				found = true
				return nil
			}
		}
	}
	return ErrNoDepositTag
}

// AnalyzeHeaderTag Analyzes the header tag
func (svc *XMLEscrowAnalysisService) AnalyzeHeaderTag() error {
	// our found flag
	found := false

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Looking for header tag in %s ... \n", svc.Deposit.FileName)
	for {
		if found {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return errors.Join(ErrDecodingToken, tokenErr)
		}
		// Check the type
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "header" {
				if err := d.DecodeElement(&svc.Header, &se); err != nil {
					return fmt.Errorf("error decoding header: %s", tokenErr)
				}
				found = true
				return nil
			}
		}
	}
	return ErrNoHeaderTag
}

// AnalyzeRegistrarTags Gets all registrars from the escrow file
func (svc *XMLEscrowAnalysisService) AnalyzeRegistrarTags(expectedRegistrarCount int) error {

	count := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Looking up %d registrars in %s ... \n", expectedRegistrarCount, svc.Deposit.FileName)

	for {
		if count == expectedRegistrarCount {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type registrar
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "registrar" {
				// Skip registrar elements that are not in the registrar namespace
				if se.Name.Space != entities.REGISTRAR_URI {
					continue
				}
				var registrar entities.RDERegistrar
				if err := d.DecodeElement(&registrar, &se); err != nil {
					return fmt.Errorf("error decoding registrar: %s", tokenErr)
				}
				// Add registrars to our inventory
				svc.Registrars = append(svc.Registrars, registrar)
				// Create an empty RdeRegistrarInfo counter for each registrar in our Mapping.
				// We will populate these counters when going through the deposit and find objects that belong to this registrar
				svc.RegsistrarMapping[registrar.ID] = entities.RdeRegistrarInfo{
					Name:  registrar.Name,
					GurID: registrar.GurID,
				}
				count++
			}
		}
	}
	return nil
}

// AnalyzeIDNTableRefs decodes and saves all IDN table references from the escrow file
func (svc *XMLEscrowAnalysisService) AnalyzeIDNTableRefTags(idnCount int) error {
	var idnTableRefs []entities.RDEIdnTableReference

	count := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Looking up %d IDN table references... \n", idnCount)

	for {
		if count == idnCount {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return errors.Join(ErrDecodingToken, tokenErr)
		}

		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "idnTableRef" {
				var idnTableRef entities.RDEIdnTableReference
				if err := d.DecodeElement(&idnTableRef, &se); err != nil {
					return fmt.Errorf("error decoding IDN table ref: %s", tokenErr)
				}
				idnTableRefs = append(idnTableRefs, idnTableRef)
				count++
			}
		}
	}
	svc.IDNs = idnTableRefs
	return nil
}

// ExtractContacts Extracts contacts from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-contacts.csv
// - {inputFilename}-contactStatuses.csv
// - {inputFilename}-contactPostalInfo.csv
func (svc *XMLEscrowAnalysisService) ExtractContacts() error {

	count := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	// Prepare the CSV file to receive the contacts
	outFileName := svc.GetDepositFileNameWoExtension() + "-contacts.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	contactWriter := csv.NewWriter(outFile)

	// Prepare the CSV file to receive the contact statuses
	statusFileName := svc.GetDepositFileNameWoExtension() + "-contactStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return err
	}
	statusWriter := csv.NewWriter(statusFile)
	statusCounter := 0

	// Prepare the CSV file to receive the contact postal info
	postalInfoFileName := svc.GetDepositFileNameWoExtension() + "-contactPostalInfo.csv"
	postalInfoFile, err := os.Create(postalInfoFileName)
	if err != nil {
		return err
	}
	postalInfoWriter := csv.NewWriter(postalInfoFile)
	postalInfoCounter := 0

	fmt.Printf("Looking up %d contacts... \n", svc.Header.ContactCount())
	pbar := progressbar.Default(-1)

	for {
		// Stop when we have found all contacts
		if count == svc.Header.ContactCount() {
			break
		}

		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return fmt.Errorf("error decoding token: %s", tokenErr)
		}
		// Only process start elements of type contact
		switch se := t.(type) {
		case xml.StartElement:
			pbar.Add(1)
			if se.Name.Local == "contact" {
				// Skip contacts that are not in the contact namespace
				if se.Name.Space != entities.CONTACT_URI {
					continue
				}
				var contact entities.RDEContact
				if err := d.DecodeElement(&contact, &se); err != nil {
					return fmt.Errorf("error decoding contact: %s", tokenErr)
				}
				// Write the contact to the contact file
				contactWriter.Write([]string{contact.ID, contact.RoID, contact.Voice, contact.Fax, contact.Email, contact.ClID, contact.CrRr, contact.CrDate, contact.UpRr, contact.UpDate})
				// Set Status in statusFile
				cStatuses := []string{contact.ID}
				for _, status := range contact.Status {
					cStatuses = append(cStatuses, status.S)
				}
				for i, s := range cStatuses {
					if i == 0 {
						continue
					}
					statusCounter++
					statusWriter.Write([]string{contact.ID, s})
				}
				// Set postalInfo in postalInfoFile
				cPostalInfo := make(map[int][]string)
				for i, postalInfo := range contact.PostalInfo {
					postalInfoCounter++
					cPostalInfo[i] = append(cPostalInfo[i], contact.ID)
					cPostalInfo[i] = append(cPostalInfo[i], postalInfo.Type, postalInfo.Org)
					// This is clunky but we need to ensure there are always 3 Street elements for CSV length consistency
					// First add the ones that are there
					cPostalInfo[i] = append(cPostalInfo[i], postalInfo.Address.Street...)
					// Then add empty strings for the ones that are missing
					for i := 3 - len(postalInfo.Address.Street); i == 0; i-- {
						cPostalInfo[i] = append(cPostalInfo[i], "")
					}
					cPostalInfo[i] = append(cPostalInfo[i], postalInfo.Address.City, postalInfo.Address.StateProvince, postalInfo.Address.PostalCode, postalInfo.Address.CountryCode)
				}

				for _, v := range cPostalInfo {
					postalInfoWriter.Write(v)
				}

				// Update counters in Registrar Map
				objCount := svc.RegsistrarMapping[contact.ClID]
				objCount.ContactCount++
				svc.RegsistrarMapping[contact.ClID] = objCount
				count++
			}
		}
	}
	fmt.Println("Done!")
	if postalInfoCounter < svc.Header.ContactCount() {
		log.Printf("🔥 WARNING 🔥 Expected at least %d postalInfo objects, but found %d\n", svc.Header.ContactCount(), postalInfoCounter)
	}
	if statusCounter < svc.Header.ContactCount() {
		log.Printf("🔥 WARNING 🔥 Expected at least %d status objects, but found %d\n", svc.Header.ContactCount(), statusCounter)
	}
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	postalInfoWriter.Flush()
	checkLineCount(postalInfoFileName, postalInfoCounter)
	contactWriter.Flush()
	checkLineCount(outFileName, svc.Header.ContactCount())
	return nil
}

// getXMLDecoder opens the XML file and returns an XML decoder
func (svc *XMLEscrowAnalysisService) getXMLDecoder() (*xml.Decoder, error) {
	f, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return nil, err
	}
	return xml.NewDecoder(f), nil
}

// GetDepositFileNameWoExtension Returns the XML Deposit Filename without exitension
func (svc *XMLEscrowAnalysisService) GetDepositFileNameWoExtension() string {
	return strings.Join(strings.Split(svc.Deposit.FileName, ".")[0:len(strings.Split(svc.Deposit.FileName, "."))-1], ".")
}

// Checks if the number of lines in the file matches the expected number
func checkLineCount(filename string, expected int) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0444)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	lineCount, err := CountLines(file)
	if err != nil {
		log.Fatal(err)
	}
	if lineCount != expected {
		log.Printf("🔥 WARNING 🔥 Expecting %d objects, found %d objects in %s \n", expected, lineCount, filename)
		if lineCount > expected {
			log.Println(`This might indicate there are newline(\n) characters in the data.`)
		}
	} else {
		log.Printf("✅ All %d objects were extracted to %s \n", expected, filename)
	}
}

// Count the number of lines in a file by looking for \n occurrences. Use this to check against the number of objects in the header
func CountLines(r io.Reader) (int, error) {
	var count int
	var read int
	var err error
	var target []byte = []byte("\n")

	buffer := make([]byte, 32*1024)

	for {
		read, err = r.Read(buffer)
		if err != nil {
			break
		}

		count += bytes.Count(buffer[:read], target)
	}

	if err == io.EOF {
		return count, nil
	}

	return count, err
}
