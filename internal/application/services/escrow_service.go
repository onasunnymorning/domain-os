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
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/schollz/progressbar/v3"
)

const (
	CONCURRENT_CLIENTS = 10
)

var (
	ErrDecodingToken                      = errors.New("error decoding token")
	ErrNoDepositTag                       = errors.New("no deposit tag found")
	ErrNoHeaderTag                        = errors.New("no header tag found")
	ErrDecodingXML                        = errors.New("error decoding XML")
	ErrAnalysisContainsErrors             = errors.New("analysis shows errors")
	ErrAnalysisFileDoesNotMatchEscrowFile = errors.New("analysis file does not match escrow file")
	ErrImportFailed                       = errors.New("import failed, at least one object could not be imported")

	BASE_URL = "http://" + os.Getenv("API_HOST") + ":" + os.Getenv("API_PORT")
	BEARER   = "Bearer " + os.Getenv("API_TOKEN")
)

// XMLEscrowService implements XMLEscrowService interface
type XMLEscrowService struct {
	Deposit          entities.RDEDeposit             `json:"deposit"`
	Header           entities.RDEHeader              `json:"header"`
	Registrars       []entities.RDERegistrar         `json:"registrars"`
	IDNs             []entities.RDEIdnTableReference `json:"idns"`
	RegistrarMapping entities.RegistrarMapping       `json:"registrarMapping"`
	Analysis         entities.EscrowAnalysis         `json:"analysis"`
	Import           entities.EscrowImport           `json:"import"`
	uniqueContactIDs map[string]bool                 `json:"-"`
}

// NewXMLEscrowService creates a new instance of EscrowService
func NewXMLEscrowService(XMLFilename string) (*XMLEscrowService, error) {
	// Fail fast if we can't open the file
	f, err := os.Open(XMLFilename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	d := XMLEscrowService{}
	// Set filename and size
	d.Deposit.FileName = XMLFilename
	d.Deposit.FileSize = fi.Size()
	log.Printf("Escow file %s is %d MB\n", XMLFilename, d.Deposit.FileSize/1024/1024)

	// Initialize the registrar mapping
	d.RegistrarMapping = entities.NewRegistrarMapping()

	// Initialize the unique contact IDs map
	d.uniqueContactIDs = make(map[string]bool)

	return &d, nil
}

// GetDeposit returns the RdeDeposit element in JSON format
func (svc *XMLEscrowService) GetDepositJSON() string {
	jsonDepositBytes, err := json.MarshalIndent(svc.Deposit, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonDepositBytes)
}

// GetHeader returns the RdeHeader element in JSON format
func (svc *XMLEscrowService) GetHeaderJSON() string {
	jsonHeaderBytes, err := json.MarshalIndent(svc.Header, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonHeaderBytes)
}

// Analyzes the deposit XML tag
func (svc *XMLEscrowService) AnalyzeDepostTag() error {
	// our found flag
	found := false

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Analyzing deposit tag in %s (this may take a while for a large file) ... \n", svc.Deposit.FileName)
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
					return errors.Join(ErrDecodingXML, err)
				}
				found = true
				return nil
			}
		}
	}
	return ErrNoDepositTag
}

// AnalyzeHeaderTag Analyzes the header tag
func (svc *XMLEscrowService) AnalyzeHeaderTag() error {
	// our found flag
	found := false

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Analyzing header tag in %s ... \n", svc.Deposit.FileName)
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
					return errors.Join(ErrDecodingXML, err)
				}
				found = true
				return nil
			}
		}
	}
	return ErrNoHeaderTag
}

// AnalyzeRegistrarTags Gets all registrars from the escrow file
func (svc *XMLEscrowService) AnalyzeRegistrarTags(expectedRegistrarCount int) error {

	count := 0
	errCount := 0

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
					return errors.Join(ErrDecodingXML, err)
				}
				// Pass it through our entity for validation (if we have a valid escrow that doesn't messup 'int' and 'loc' postalinfo)
				_, err = registrar.ToEntity()
				if err != nil {
					errCount++
					svc.Analysis.Warnings = append(svc.Analysis.Errors, fmt.Sprintf("Error parsing registrar entity for %s: %s", registrar.Name, err))
				}
				// Add registrars to our inventory
				svc.Registrars = append(svc.Registrars, registrar)
				// Create an empty RdeRegistrarInfo counter for each registrar in our Mapping.
				// We will populate these counters when going through the deposit and find objects that belong to this registrar
				svc.RegistrarMapping[registrar.ID] = entities.RdeRegistrarInfo{
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
func (svc *XMLEscrowService) AnalyzeIDNTableRefTags(idnCount int) error {
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
					return errors.Join(ErrDecodingXML, err)
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
func (svc *XMLEscrowService) ExtractContacts(returnCommands bool) ([]commands.CreateContactCommand, error) {

	count := 0
	unlinkedCount := 0
	errCount := 0
	createCommands := []commands.CreateContactCommand{}

	d, err := svc.getXMLDecoder()
	if err != nil {
		return nil, err
	}

	// Prepare the CSV file to receive the contacts
	outFileName := svc.GetDepositFileNameWoExtension() + "-contacts.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()
	contactWriter := csv.NewWriter(outFile)

	// Prepare the CSV file to receive the contact statuses
	statusFileName := svc.GetDepositFileNameWoExtension() + "-contactStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return nil, err
	}
	statusWriter := csv.NewWriter(statusFile)
	statusCounter := 0

	// Prepare the CSV file to receive the contact postal info
	postalInfoFileName := svc.GetDepositFileNameWoExtension() + "-contactPostalInfo.csv"
	postalInfoFile, err := os.Create(postalInfoFileName)
	if err != nil {
		return nil, err
	}
	postalInfoWriter := csv.NewWriter(postalInfoFile)
	postalInfoCounter := 0

	pbar := progressbar.Default(int64(svc.Header.ContactCount()), "Reading Contacts from XML")

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
			return nil, errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type contact
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "contact" {
				// Skip contacts that are not in the contact namespace
				if se.Name.Space != entities.CONTACT_URI {
					continue
				}
				var rdeContact entities.RDEContact
				if err := d.DecodeElement(&rdeContact, &se); err != nil {
					return nil, errors.Join(ErrDecodingXML, err)
				}

				// Only process linked contacts.
				// Sometimes contacts have status linked, but are not linked to a domain in this deposit. In this case we will also skip them
				if rdeContact.IsLinked() && svc.uniqueContactIDs[rdeContact.ID] {
					// Validate using a CreateContactCommand
					cmd := commands.CreateContactCommand{}
					err = cmd.FromRdeContact(&rdeContact)
					if err != nil {
						errCount++
						svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error creating contact command for %s: %s", rdeContact.ID, err))
					}
					// Add the command to our slice of create commands, if required AND the contact is linked, otherwise no need to import it
					if returnCommands && cmd.Status.Linked {
						createCommands = append(createCommands, cmd)
					}

					// Write the contact to the contact file
					contactWriter.Write(rdeContact.ToCSV())
					// Set Status in statusFile
					cStatuses := []string{rdeContact.ID}
					for _, status := range rdeContact.Status {
						cStatuses = append(cStatuses, status.S)
					}
					for i, s := range cStatuses {
						if i == 0 {
							continue
						}
						statusCounter++
						statusWriter.Write([]string{rdeContact.ID, s})
					}
					// Set postalInfo in postalInfoFile
					cPostalInfo := make(map[int][]string)
					for i, postalInfo := range rdeContact.PostalInfo {
						postalInfoCounter++
						cPostalInfo[i] = append(cPostalInfo[i], rdeContact.ID)         // Add the contact ID as the first element
						cPostalInfo[i] = append(cPostalInfo[i], postalInfo.ToCSV()...) // Add the postal info
					}

					for _, v := range cPostalInfo {
						postalInfoWriter.Write(v)
					}

					// Update counters in Registrar Map
					objCount := svc.RegistrarMapping[rdeContact.ClID]
					objCount.ContactCount++
					svc.RegistrarMapping[rdeContact.ClID] = objCount
					count++
				} else {
					unlinkedCount++
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Unlinked contact %s will not be imported", rdeContact.ID))
				}

				pbar.Add(1)
			}
		}
	}
	log.Println("Done!")
	if unlinkedCount > 0 {
		log.Printf("🔥 WARNING 🔥 %d unlinked contacts were found in the escrow file and will not be imported\n", unlinkedCount)
	}
	if postalInfoCounter < svc.Header.ContactCount()-unlinkedCount {
		log.Printf("🔥 WARNING 🔥 Expected at least %d postalInfo objects, but found %d\n", svc.Header.ContactCount(), postalInfoCounter)
	}
	if statusCounter < svc.Header.ContactCount()-unlinkedCount {
		log.Printf("🔥 WARNING 🔥 Expected at least %d status objects, but found %d\n", svc.Header.ContactCount(), statusCounter)
	}
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	postalInfoWriter.Flush()
	checkLineCount(postalInfoFileName, postalInfoCounter)
	contactWriter.Flush()
	checkLineCount(outFileName, svc.Header.ContactCount()-unlinkedCount)
	if errCount > 0 {
		log.Printf("🔥 WARNING 🔥 %d errors were encountered while processing contacts. See analysis file for details\n", errCount)
	}
	return createCommands, nil
}

// ExtractHosts Extracts hosts from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-hosts.csv
// - {inputFilename}-hostStatuses.csv
// - {inputFilename}-hostAddresses.csv
func (svc *XMLEscrowService) ExtractHosts(returnHostCommands bool) ([]commands.CreateHostCommand, error) {

	count := 0
	unlinkedCount := 0
	errCount := 0
	hostCmds := []commands.CreateHostCommand{}

	f, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := xml.NewDecoder(f)

	// Prepare the CSV file to receive the hosts
	outFileName := svc.GetDepositFileNameWoExtension() + "-hosts.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()
	writer := csv.NewWriter(outFile)

	// Prepare the CSV file to receive the host statuses
	statusFileName := svc.GetDepositFileNameWoExtension() + "-hostStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return nil, err
	}
	defer statusFile.Close()
	statusWriter := csv.NewWriter(statusFile)

	// Prepare the CSV file to receive the host addresses
	addrFileName := svc.GetDepositFileNameWoExtension() + "-hostAddresses.csv"
	addrFile, err := os.Create(addrFileName)
	if err != nil {
		return nil, err
	}
	defer addrFile.Close()
	addrWriter := csv.NewWriter(addrFile)
	addrCounter := 0
	statusCounter := 0

	pbar := progressbar.Default(int64(svc.Header.HostCount()), "Reading Hosts from XML")

	for {
		if count == svc.Header.HostCount() {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return nil, errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type host
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "host" {
				// Skip hosts that are not in the host namespace
				if se.Name.Space != entities.HOST_URI {
					continue
				}
				var host entities.RDEHost
				if err := d.DecodeElement(&host, &se); err != nil {
					return nil, errors.Join(ErrDecodingXML, err)
				}

				// Flag unlinked hosts
				if !host.IsLinked() {
					unlinkedCount++
				}

				// Validate using a CreateHostCommand
				cmd := commands.CreateHostCommand{}
				err = cmd.FromRdeHost(&host)
				if err != nil {
					errCount++
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Error creating host command for %s: %s", host.Name, err))
				}

				writer.Write(host.ToCSV())
				// Set Status in statusFile
				hStatuses := []string{host.Name}
				for _, status := range host.Status {
					statusCounter++
					hStatuses = append(hStatuses, status.S)
				}
				for i, s := range hStatuses {
					if i == 0 {
						continue
					}
					statusWriter.Write([]string{host.Name, s})
				}
				// Set addresses in addrFile
				for _, addr := range host.Addr {
					addrCounter++
					addrWriter.Write([]string{host.Name, addr.IP, addr.ID})
				}

				// Add the command to our slice of create commands
				if returnHostCommands {
					// Unset the linked status in case the deposit claims its linked but there is no domain linked to it.
					// Once we linke the hosts and domains the status will be updated
					cmd.Status.Linked = false
					hostCmds = append(hostCmds, cmd)
				}

				// Update counters in Registrar Map
				objCount := svc.RegistrarMapping[host.ClID]
				objCount.HostCount++
				svc.RegistrarMapping[host.ClID] = objCount
				count++

				pbar.Add(1)
			}
		}
	}
	log.Println("Done!")
	if unlinkedCount > svc.Header.HostCount()/10 {
		log.Printf("🔥 WARNING 🔥 %d more than 1/10th hosts are unlinked, but can still be imported\n", unlinkedCount)
	}
	addrWriter.Flush()
	checkLineCount(addrFileName, addrCounter)
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	writer.Flush()
	checkLineCount(outFileName, svc.Header.HostCount())
	if errCount > 0 {
		log.Printf("🔥 WARNING 🔥 %d errors were encountered while processing hosts. See analysis file for details\n", errCount)
	}
	return hostCmds, nil
}

// ExtractNNDNS Extracts statuses from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-nndns.csv
func (svc *XMLEscrowService) ExtractNNDNS(returnNNDNCreateCommands bool) ([]commands.CreateNNDNCommand, error) {

	count := 0
	errCount := 0
	nndnCmds := []commands.CreateNNDNCommand{}

	d, err := svc.getXMLDecoder()
	if err != nil {
		return nil, err
	}

	// Prepare the CSV file to receive the nndns
	outFileName := svc.GetDepositFileNameWoExtension() + "-nndns.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)

	pbar := progressbar.Default(int64(svc.Header.NNDNCount()), "Reading NNDNs from XML")

	for {
		if count == svc.Header.NNDNCount() {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return nil, errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type nndn
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "NNDN" {
				// Skip nndns that are not in the nndns namespace
				if se.Name.Space != entities.NNDN_URI {
					continue
				}
				var rdeNNDN entities.RDENNDN
				if err := d.DecodeElement(&rdeNNDN, &se); err != nil {
					return nil, errors.Join(ErrDecodingXML, err)
				}

				// Validate using a CreateNNDNCommand
				cmd := commands.CreateNNDNCommand{}
				err = cmd.FromRDENNDN(&rdeNNDN)
				if err != nil {
					errCount++
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Error creating NNDN command for %s: %s", rdeNNDN.AName, err))
				}

				if returnNNDNCreateCommands {
					// Add the command to our slice of create commands
					nndnCmds = append(nndnCmds, cmd)
				}

				// Write to CSV
				writer.Write(rdeNNDN.ToCSV())
				count++

				pbar.Add(1)
			}
		}
	}
	writer.Flush()
	checkLineCount(outFileName, svc.Header.NNDNCount())
	if errCount > 0 {
		log.Printf("🔥 WARNING 🔥 %d errors were encountered while processing NNDNs. See analysis file for details\n", errCount)
	}
	return nndnCmds, nil
}

// getXMLDecoder opens the XML file and returns an XML decoder
func (svc *XMLEscrowService) getXMLDecoder() (*xml.Decoder, error) {
	f, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return nil, err
	}
	return xml.NewDecoder(f), nil
}

// GetDepositFileNameWoExtension Returns the XML Deposit Filename without exitension
func (svc *XMLEscrowService) GetDepositFileNameWoExtension() string {
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
	var tip = ""
	if lineCount != expected {
		if lineCount > expected {
			tip = `This might indicate there are newline(\n) characters in the data.`
		}
		log.Printf("🔥 WARNING 🔥 Expecting %d objects, found %d objects in %s %s\n", expected, lineCount, filename, tip)
	} else {
		log.Printf("✅ All %d objects were extracted to %s \n", expected, filename)
	}
}

// ExtractDomains Extracts domains from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-domains.csv
// - {inputFilename}-domainStatuses.csv
// - {inputFilename}-domainNameservers.csv
// - {inputFilename}-DomainDnssec.csv
// - {inputFilename}-domainTransfers.csv
// - {inputFilename}-uniqueDomainContactIDs.csv
func (svc *XMLEscrowService) ExtractDomains(returnCommands bool) ([]commands.CreateDomainCommand, error) {

	count := 0
	domCreateCommands := []commands.CreateDomainCommand{}
	errCount := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return nil, err
	}

	// Create a CSV file and writer to write the main domain information to
	outFileName := svc.GetDepositFileNameWoExtension() + "-domains.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()
	domainWriter := csv.NewWriter(outFile)

	// Create a -status CSV file and writer to write the domain statuses to
	statusFileName := svc.GetDepositFileNameWoExtension() + "-domainStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return nil, err
	}
	statusWriter := csv.NewWriter(statusFile)
	statusCounter := 0

	// Create a -nameserver CSV file and writer to write the nameservers to
	nameserverFileName := svc.GetDepositFileNameWoExtension() + "-domainNameservers.csv"
	nameserverFile, err := os.Create(nameserverFileName)
	if err != nil {
		return nil, err
	}
	nameserverWriter := csv.NewWriter(nameserverFile)
	nameServerCounter := 0

	// Create a -dnssec CSV file and writer to write the dnssec information to
	dnssecFileName := svc.GetDepositFileNameWoExtension() + "-DomainDnssec.csv"
	dnssecFile, err := os.Create(dnssecFileName)
	if err != nil {
		return nil, err
	}
	dnssecWriter := csv.NewWriter(dnssecFile)
	dnssecCounter := 0

	// Create a -transfers CSV file and writer to write the transfer information to
	transferFileName := svc.GetDepositFileNameWoExtension() + "-domainTransfers.csv"
	transferFile, err := os.Create(transferFileName)
	if err != nil {
		return nil, err
	}
	transferWriter := csv.NewWriter(transferFile)
	transferCounter := 0

	// Create a file and writer to write the unique contact IDs to
	contactIDFileName := svc.GetDepositFileNameWoExtension() + "-uniqueDomainContactIDs.csv"
	contactIDFile, err := os.Create(contactIDFileName)
	if err != nil {
		return nil, err
	}
	contactIDWriter := csv.NewWriter(contactIDFile)

	pbar := progressbar.Default(int64(svc.Header.DomainCount()), "Reading Domains from XML")

	for {
		if count == svc.Header.DomainCount() {
			break
		}
		// Read the next token
		t, tokenErr := d.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return nil, errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type domain
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "domain" {
				// Skip domains that are not in the domain namespace
				if se.Name.Space != entities.DOMAIN_URI {
					continue
				}
				var dom entities.RDEDomain
				if err := d.DecodeElement(&dom, &se); err != nil {
					return nil, errors.Join(ErrDecodingXML, err)
				}

				// Validate using a CreateDomainCommand
				cmd := commands.CreateDomainCommand{}
				err = cmd.FromRdeDomain(&dom)
				if err != nil {
					errCount++
					svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("error creating domain command for %s: %s", dom.Name, err))
				}

				if returnCommands {
					domCreateCommands = append(domCreateCommands, cmd)
				}

				// Write the domain to the domain file
				domainWriter.Write(dom.ToCSV())
				// Add a line to the contactID file for each contact, only if it does not exist yet
				// Start with the registrant
				if !svc.uniqueContactIDs[dom.Registrant] {
					svc.uniqueContactIDs[dom.Registrant] = true
				}
				// Now loop over the contacts
				for _, contact := range dom.Contact {
					// Only add it if it is not there already
					if !svc.uniqueContactIDs[contact.ID] {
						svc.uniqueContactIDs[contact.ID] = true
					}
				}
				// Write the domain statuses to the status file
				dStatuses := []string{dom.Name.String()}
				for _, status := range dom.Status {
					dStatuses = append(dStatuses, status.S)
				}
				for i, s := range dStatuses {
					if i == 0 {
						continue
					}
					statusCounter++
					statusWriter.Write([]string{dom.Name.String(), s})
				}
				// Write the nameservers to the nameserver file
				dNameservers := []string{dom.Name.String()}
				for _, ns := range dom.Ns {
					dNameservers = append(dNameservers, ns.HostObjs...)
				}
				for i, ns := range dNameservers {
					if i == 0 {
						continue
					}
					nameServerCounter++
					nameserverWriter.Write([]string{dom.Name.String(), ns})
				}
				// Write the dnssec information to the dnssec file
				for _, dsData := range dom.SecDNS.DSData {
					dnssecCounter++
					dnssecWriter.Write([]string{dom.Name.String(), strconv.Itoa(dsData.KeyTag), strconv.Itoa(dsData.Alg), strconv.Itoa(dsData.DigestType), dsData.Digest})
				}
				// Write the transfer information to the transfer file
				if dom.TrnData.TrStatus.State != "" {
					transferCounter++
					transferWriter.Write([]string{dom.Name.String(), dom.TrnData.TrStatus.State, dom.TrnData.ReRr.RegID, dom.TrnData.ReDate, dom.TrnData.ReRr.RegID, dom.TrnData.AcDate, dom.TrnData.ExDate})
				}

				// Update counters in Registrar Map
				objCount := svc.RegistrarMapping[dom.ClID]
				objCount.DomainCount++
				svc.RegistrarMapping[dom.ClID] = objCount
				count++

				pbar.Add(1)
			}
		}
	}
	// Write the unique contact IDs to the contactID file
	for k := range svc.uniqueContactIDs {
		contactIDWriter.Write([]string{k})
	}
	contactIDWriter.Flush()
	log.Printf("✅  Written %d unique contact IDs used by Domains to : %s", len(svc.uniqueContactIDs), contactIDFileName)
	log.Println("Done!")
	if statusCounter < svc.Header.DomainCount() {
		log.Printf("🔥 WARNING 🔥 Expected at least %d status objects, but found %d\n", svc.Header.DomainCount(), statusCounter)
	}
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	nameserverWriter.Flush()
	checkLineCount(nameserverFileName, nameServerCounter)
	dnssecWriter.Flush()
	checkLineCount(dnssecFileName, dnssecCounter)
	transferWriter.Flush()
	checkLineCount(transferFileName, transferCounter)
	domainWriter.Flush()
	checkLineCount(outFileName, svc.Header.DomainCount())
	if errCount > 0 {
		log.Printf("🔥 WARNING 🔥 %d errors were encountered while processing domains. See analysis file for details\n", errCount)
	}
	return domCreateCommands, nil
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

// getUniqueContactIDs Extracts the contact IDs from the contact file and returns them as a map
func (svc *XMLEscrowService) getUniqueContactIDs() (map[string]bool, error) {
	contactIDs := make(map[string]bool)
	f, err := os.Open(svc.GetDepositFileNameWoExtension() + "-contacts.csv")
	if err != nil {
		return contactIDs, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return contactIDs, err
	}
	for _, record := range records {
		contactIDs[record[0]] = true
	}
	return contactIDs, nil
}

// LoadUniqueContactIDs Loads the unique contact IDs from the contact file
func (svc *XMLEscrowService) LoadUniqueContactIDs() error {
	var err error
	svc.uniqueContactIDs, err = svc.getUniqueContactIDs()
	if err != nil {
		return err
	}
	return nil
}

// LookForMissingContacts Looks if all the uniqueContactIDs used on domains are present in the contact file. It saves the results in the escrow object
func (svc *XMLEscrowService) LookForMissingContacts() error {
	var err error
	svc.uniqueContactIDs, err = svc.getUniqueContactIDs()
	if err != nil {
		return err
	}
	f, err := os.Open(svc.GetDepositFileNameWoExtension() + "-uniqueDomainContactIDs.csv")
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}
	errorCount := 0
	missingContactIDs := []string{}
	for _, record := range records {
		if !svc.uniqueContactIDs[record[0]] {
			errorCount++
			missingContactIDs = append(missingContactIDs, record[0])
		}
	}
	if errorCount > 0 {
		svc.Analysis.MissingContacts = missingContactIDs
		log.Printf("🔥 WARNING 🔥 Found %d missing contact IDs in %s \n", errorCount, svc.GetDepositFileNameWoExtension()+"-uniqueDomainContactIDs.csv")
	} else {
		log.Printf("✅ Found all domain contact IDs in the contact file\n")

	}
	return nil
}

// UnlinkedContactCheck Checks if the number of contacts is more than 4 times the number of domains. This could indicate unlinked contacts. This requires the header to be analyzed before use.
func (svc *XMLEscrowService) UnlinkedContactCheck() {
	if svc.Header.ContactCount() > svc.Header.DomainCount()*4 {
		log.Println("🔥 WARNING 🔥 Deposit contains more contacts than four times the number of domains, this could indicate the presence of unlinked contacts in the deposit")
	}
}

// Save Analysis to a JSON file
func (svc *XMLEscrowService) SaveAnalysis() error {
	bytes, err := json.MarshalIndent(svc, "", "	")
	if err != nil {
		return err
	}
	analysisFileName := svc.GetDepositFileNameWoExtension() + "-analysis.json"
	os.WriteFile(analysisFileName, bytes, 0644)
	log.Printf("✅  Saved analysis to: %s\n", analysisFileName)
	return nil
}

// Save the Import Struct to a JSON file
func (svc *XMLEscrowService) SaveImportResult() error {
	bytes, err := json.MarshalIndent(svc.Import, "", "	")
	if err != nil {
		return err
	}
	importFileName := svc.GetDepositFileNameWoExtension() + "-import.json"
	os.WriteFile(importFileName, bytes, 0644)
	log.Printf("✅  Saved import to: %s\n", importFileName)
	return nil
}

// MapRegistrars Tries to find all registrars from the deposit in the repository through the registrars API and link their IDs
func (svc *XMLEscrowService) MapRegistrars() error {
	log.Println("Mapping Registrars ...")
	if svc.Registrars == nil {
		return errors.New("no registrars to map, have you analyzed registrar-section of the escrow file?")
	}

	outFile, err := os.Create(svc.GetDepositFileNameWoExtension() + "-registrarMapping.csv")
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	bearer := "Bearer " + os.Getenv("API_TOKEN")

	var found = 0
	var missing = 0
	var missingGurIDs = []int{}

	for _, rar := range svc.Registrars {

		var URL string

		// Handle special cases of reserved GurIDs
		if rar.GurID == 9997 {
			URL = BASE_URL + "/registrars/9997-ICANN-SLAM"
		} else if rar.GurID == 9995 {
			URL = BASE_URL + "/registrars/9995-ICANN-RST"
		} else if rar.GurID == 9998 {
			URL = BASE_URL + "/registrars/9998" + "." + strings.ToLower(svc.Header.TLD)
		} else if rar.GurID == 9999 || rar.GurID == 119 || rar.GurID == 0 { // TODO: FIXME: 0 => 9999 mapping is okay for gTLDs since we can't have domains under these, but a bit dangerous, should be handled better
			URL = BASE_URL + "/registrars/9999" + "." + strings.ToLower(svc.Header.TLD)
		} else {
			URL = BASE_URL + "/registrars/gurid/" + strconv.Itoa((rar.GurID))
		}

		req, err := http.NewRequest("GET", URL, nil)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", bearer)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		// not found
		if resp.StatusCode == 404 {
			// If the regsitrar is in the deposit but not found, we can skip it if it has no domains
			if svc.RegistrarMapping[rar.ID].DomainCount == 0 {
				if svc.RegistrarMapping[rar.ID].HostCount == 0 && svc.RegistrarMapping[rar.ID].ContactCount == 0 {
					log.Printf("Registrar %s with GurID %d not found, but has no objects, skipping ...", rar.Name, rar.GurID)
					continue
				}
				svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Registrar %s with GurID %d not found. Has no domains, but %d hosts and %d contacts", rar.Name, rar.GurID, svc.RegistrarMapping[rar.ID].HostCount, svc.RegistrarMapping[rar.ID].ContactCount))
			}
			missing++
			missingGurIDs = append(missingGurIDs, rar.GurID)
			continue
		}

		defer resp.Body.Close()

		// success
		if resp.StatusCode == 200 {
			var responseRar entities.Registrar
			err = json.NewDecoder(resp.Body).Decode(&responseRar)
			if err != nil {
				log.Printf("error decoding registrar: %s", err)
			}
			// update mapping
			rarMap := svc.RegistrarMapping[rar.ID]
			rarMap.Name = rar.Name
			rarMap.GurID = rar.GurID
			rarMap.RegistrarClID = responseRar.ClID
			svc.RegistrarMapping[rar.ID] = rarMap
			found++
			continue
		}

		// other error
		log.Printf("got a %s: %s", resp.Status, URL)
		missing++
		missingGurIDs = append(missingGurIDs, rar.GurID)

	}
	// write mapping to file
	for k, v := range svc.RegistrarMapping {
		writer.Write([]string{k, v.Name, strconv.Itoa(v.GurID), v.RegistrarClID.String(), strconv.Itoa(v.DomainCount), strconv.Itoa(v.HostCount), strconv.Itoa(v.ContactCount)})
	}

	if missing > 0 {
		log.Printf("🔥 WARNING 🔥 Could not map %d registrars\n", missing)
		log.Printf("The following registrars' GurIDs could not be found: %v\n", missingGurIDs)
	} else {
		log.Printf("✅ Found all important registrars\n")
	}

	return nil
}

// Loads the analysis file produced by the escrow analyzer. Input should be provided by the user
func (svc *XMLEscrowService) LoadDepostiAnalysis(analysisFile, escrowFile string) error {
	f, err := os.Open(analysisFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read the contents of the file into a byte slice
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Unmarshal the file contents into the Analysis struct
	err = json.Unmarshal(data, &svc)
	if err != nil {
		return err
	}

	if svc.Deposit.FileName != escrowFile {
		return ErrAnalysisFileDoesNotMatchEscrowFile
	}

	log.Println("Analysis file loaded successfully")

	if len(svc.Analysis.Errors) != 0 {
		log.Printf("🔥 WARNING 🔥 the analysis file shows there are %d errors", len(svc.Analysis.Errors))
		// for _, e := range svc.Analysis.Errors {
		// 	log.Println(e)
		// }

		log.Println("Cannot proceed with import due to errors in the analysis file. Please fix the errors and try again.")
		return ErrAnalysisContainsErrors
	}

	if len(svc.Analysis.Warnings) != 0 {
		log.Printf("🔥 WARNING 🔥 the analysis file shows there are %d warnings", len(svc.Analysis.Warnings))
		// for _, w := range svc.Analysis.Warnings {
		// 	log.Println(w)
		// }
		log.Println("Proceeding with import despite warnings in the analysis file.")
	}

	return nil
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// GetContactCount

// CreateContacts Creates the contacts in the repository through the Admin API
func (svc *XMLEscrowService) CreateContacts(cmds []commands.CreateContactCommand) error {
	// Create a re-usable client optimized for tcp connections
	client := getHTTPClient()

	// Create channels for sending commands
	cmdChan := make(chan commands.CreateContactCommand, len(cmds))
	wg := sync.WaitGroup{}

	// Loop over the commands and create the contacts in parrallel
	pbar := progressbar.Default(int64(len(cmds)), "Creating Contacts")

	// Start workers
	for i := 0; i < CONCURRENT_CLIENTS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range cmdChan {
				svc.createContact(*client, cmd)
				pbar.Add(1)
			}
		}()
	}

	// Send commands to the workers
	for _, cmd := range cmds {
		cmdChan <- cmd
	}
	close(cmdChan)

	// Wait for all workers to finish
	wg.Wait()

	if svc.Import.Contacts.Failed > 0 {
		log.Printf("🔥 WARNING 🔥 %d contacts failed to be created\n", svc.Import.Contacts.Failed)
		for _, e := range svc.Import.Errors {
			log.Println(e)
		}
		return nil
	}

	log.Printf("✅ Created all contacts successfully\n")
	// Do some housekeeping
	client.CloseIdleConnections()
	return nil
}

// createContact handles the actual creation of a contact through an API request. If a contact already exists, that is not an error.
func (svc *XMLEscrowService) createContact(client http.Client, cmd commands.CreateContactCommand) error {

	URL := BASE_URL + "/contacts"

	// First map the registrar ID to the registrar ClID
	registrar, ok := svc.RegistrarMapping[cmd.ClID]
	if !ok {
		svc.Import.Contacts.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.ClID))
		return nil
	}
	cmd.ClID = registrar.RegistrarClID.String()
	// Do the same for CrRR and UpRR
	if cmd.CrRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.CrRr]
		if !ok {
			svc.Import.Contacts.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.CrRr))
			return nil
		}
		cmd.CrRr = registrar.RegistrarClID.String()
	}
	if cmd.UpRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.UpRr]
		if !ok {
			svc.Import.Contacts.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.UpRr))
			return nil
		}
		cmd.UpRr = registrar.RegistrarClID.String()
	}

	// UnMarshal the command into a JSON object
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	// Send the request
	req, err := http.NewRequest("POST", URL, bytes.NewReader(jsonCmd))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", BEARER)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Success
	if resp.StatusCode == 201 {
		svc.Import.Contacts.Created++
		return nil
	}
	var response ErrorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Exists or Failed
	if resp.StatusCode == 400 {
		// if the contact already exists, we can skip it, we need to check the body for that
		// Exists, skip
		if strings.Contains(response.Error, "contact already exists") {
			svc.Import.Contacts.Existing++
			return nil
		}
	}
	svc.Import.Contacts.Failed++
	svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error creating contact with id %s: %s", cmd.ID, response.Error))

	return nil
}

// CreateHosts Creates the hosts in the repository through the Admin API
func (svc *XMLEscrowService) CreateHosts(cmds []commands.CreateHostCommand) error {
	// Create a re-usable client optimized for tcp connections
	client := getHTTPClient()

	// Create channels for sending commands
	cmdChan := make(chan commands.CreateHostCommand, len(cmds))
	wg := sync.WaitGroup{}

	// Loop over the commands and create the hosts in parrallel
	pbar := progressbar.Default(int64(len(cmds)), "Creating Hosts")

	// Start workers
	for i := 0; i < CONCURRENT_CLIENTS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range cmdChan {
				svc.createHost(*client, cmd)
				pbar.Add(1)
			}
		}()
	}

	// Send commands to the workers
	for _, cmd := range cmds {
		cmdChan <- cmd
	}
	close(cmdChan)

	// Wait for all workers to finish
	wg.Wait()

	if svc.Import.Hosts.Failed > 0 {
		log.Printf("🔥 WARNING 🔥 %d hosts failed to be created\n", svc.Import.Hosts.Failed)
		for _, e := range svc.Import.Errors {
			log.Println(e)
		}
		return nil
	}

	log.Printf("✅ Created all hosts successfully\n")
	return nil
}

// createHost handles the actual creation of a host through an API request. If a host already exists, that is not an error.
func (svc *XMLEscrowService) createHost(client http.Client, cmd commands.CreateHostCommand) error {

	URL := BASE_URL + "/hosts"

	// First map the registrar ID to the registrar ClID
	registrar, ok := svc.RegistrarMapping[cmd.ClID.String()]
	if !ok {
		svc.Import.Hosts.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar %s not found in mapping. Used as ClID on host %s ", cmd.ClID, cmd.Name))
		return nil
	}
	cmd.ClID = registrar.RegistrarClID
	// Do the same for CrRR and UpRR
	if cmd.CrRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.CrRr.String()]
		if !ok {
			svc.Import.Hosts.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar %s not found in mapping. Used as CrRr on host %s", cmd.CrRr, cmd.Name))
			return nil
		}
		cmd.CrRr = registrar.RegistrarClID
	}
	if cmd.UpRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.UpRr.String()]
		if !ok {
			svc.Import.Hosts.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar %s not found in mapping. Used as UpRr on host %s", cmd.UpRr, cmd.Name))
			return nil
		}
		cmd.UpRr = registrar.RegistrarClID
	}

	// UnMarshal the command into a JSON object
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	// Send the request
	req, err := http.NewRequest("POST", URL, bytes.NewReader(jsonCmd))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", BEARER)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Success
	if resp.StatusCode == 201 {
		var host entities.Host
		svc.Import.Hosts.Created++
		err := json.Unmarshal(body, &host)
		if err != nil {
			return err
		}
		return nil
	}

	var response ErrorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Exists or Failed
	if resp.StatusCode == 400 {
		// if the contact already exists, we can skip it, we need to check the body for that
		// Exists, skip
		if strings.Contains(response.Error, "host already exists") {
			svc.Import.Hosts.Existing++
			return nil
		}
	}

	svc.Import.Hosts.Failed++
	svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error creating host with name %s: %s", cmd.Name, response.Error))

	return nil
}

// CreateDomains Creates the contacts in the repository through the Admin API
func (svc *XMLEscrowService) CreateDomains(cmds []commands.CreateDomainCommand) error {
	// Create a re-usable client
	client := getHTTPClient()

	// Create channels for sending commands
	cmdChan := make(chan commands.CreateDomainCommand, len(cmds))
	wg := sync.WaitGroup{}

	// Loop over the commands and create the hosts in parrallel
	pbar := progressbar.Default(int64(len(cmds)), "Creating Domains")

	// Start workers
	for i := 0; i < CONCURRENT_CLIENTS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range cmdChan {
				svc.createDomain(*client, cmd)
				pbar.Add(1)
			}
		}()
	}

	// Send commands to the workers
	for _, cmd := range cmds {
		cmdChan <- cmd
	}
	close(cmdChan)

	// Wait for all workers to finish
	wg.Wait()

	if svc.Import.Domains.Failed > 0 {
		log.Printf("🔥 WARNING 🔥 %d domains failed to be created\n", svc.Import.Domains.Failed)
		for _, e := range svc.Import.Errors {
			log.Println(e)
		}
		return nil
	}

	log.Printf("✅ Created all domains successfully\n")
	return nil
}

// createDomain handles the actual creation of a domain through an API request. If a domain already exists, that is not an error.
func (svc *XMLEscrowService) createDomain(client http.Client, cmd commands.CreateDomainCommand) error {

	URL := BASE_URL + "/domains"

	// First map the registrar ID to the registrar ClID
	registrar, ok := svc.RegistrarMapping[cmd.ClID]
	if !ok {
		svc.Import.Domains.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.ClID))
		return nil
	}
	cmd.ClID = registrar.RegistrarClID.String()
	// Do the same for CrRR and UpRR
	if cmd.CrRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.CrRr]
		if !ok {
			svc.Import.Domains.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.CrRr))
			return nil
		}
		cmd.CrRr = registrar.RegistrarClID.String()
	}
	if cmd.UpRr != "" {
		registrar, ok := svc.RegistrarMapping[cmd.UpRr]
		if !ok {
			svc.Import.Domains.Failed++
			svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("registrar with ID %s not found in mapping", cmd.UpRr))
			return nil
		}
		cmd.UpRr = registrar.RegistrarClID.String()
	}

	// UnMarshal the command into a JSON object
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	// Send the request
	req, err := http.NewRequest("POST", URL, bytes.NewReader(jsonCmd))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", BEARER)
	resp, err := client.Do(req)
	if err != nil {
		svc.Import.Domains.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error sending Domain create request for %s: %s", cmd.Name, err.Error()))
		return err
	}

	// Fully read the body and close it - This is important for TCP connection reuse
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Success
	if resp.StatusCode == 201 {
		svc.Import.Domains.Created++
		return nil
	}
	// unmarshall the response
	var response ErrorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Exists or Failed
	if resp.StatusCode == 400 {
		// if the contact already exists, we can skip it, we need to check the body for that

		// Exists, skip
		if strings.Contains(response.Error, "domain already exists") {
			svc.Import.Domains.Existing++
			return nil
		}
	}
	svc.Import.Domains.Failed++
	svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error creating domain with name %s: %s", cmd.Name, response.Error))
	svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Create Domain Command: %v", cmd))

	return nil
}

// LinkHostsToDomains Links the hosts to the domains in the repository through the Admin API. It requires the domain and host objects to be created first
func (svc *XMLEscrowService) LinkHostsToDomains() error {
	client := getHTTPClient()

	// Read the CSV file that contains the mapping [domainname, hostname]
	f, err := os.Open(svc.GetDepositFileNameWoExtension() + "-domainNameservers.csv")
	if err != nil {
		return err
	}

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return err
	}

	// Create a channel for sending our requests
	cmdChan := make(chan [2]string, len(records))
	wg := sync.WaitGroup{}

	pbar := progressbar.Default(int64(len(records)), "Linking Hosts to Domains")
	// Start workers
	for i := 0; i < CONCURRENT_CLIENTS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := range cmdChan {
				svc.linkHostToDomain(*client, record[0], strings.Trim(record[1], "."))
				pbar.Add(1)
			}
		}()
	}

	// Send commands to the workers
	for _, record := range records {
		cmdChan <- [2]string{record[0], record[1]}
	}
	close(cmdChan)

	// Wait for all workers to finish
	wg.Wait()

	if svc.Import.DomainHostLinks.Missing > 0 {
		log.Printf("🔥 WARNING 🔥 %d domain-host links failed to be created\n", svc.Import.DomainHostLinks.Missing)
		for _, e := range svc.Import.Errors {
			log.Println(e)
		}
	} else {
		log.Printf("✅ Linked all hosts to domains successfully\n")
	}

	return nil
}

// linkHostToDomain links a host to a domain through the API. If the link already exists, that is not an error
func (svc *XMLEscrowService) linkHostToDomain(client http.Client, domainName, hostName string) error {

	// Create the request
	requestURL := BASE_URL + "/domains/" + domainName + "/hostname/" + hostName + "?force=true" // use force to override the domain's possible update prohibition
	req, err := http.NewRequest("POST", requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", BEARER)
	// Send it
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// Defer close and ready the body. This is important for TCP connection reuse
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == 204 {
		svc.Import.DomainHostLinks.Present++
		return nil
	} else {
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error linking host %s to domain %s: %s - %s", hostName, domainName, resp.Status, string(body)))
		svc.Import.DomainHostLinks.Missing++
	}

	return nil

}

// CreateNNDNs Creates the NNDNs in the repository through the Admin API
func (svc *XMLEscrowService) CreateNNDNs(cmds []commands.CreateNNDNCommand) error {
	// Create a re-usable client
	client := getHTTPClient()

	// Create channels for sending commands
	cmdChan := make(chan commands.CreateNNDNCommand, len(cmds))
	wg := sync.WaitGroup{}

	// Loop over the commands and create the hosts in parrallel
	pbar := progressbar.Default(int64(len(cmds)), "Creating NNDNs")

	// Start workers
	for i := 0; i < CONCURRENT_CLIENTS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range cmdChan {
				svc.createNNDN(*client, cmd)
				pbar.Add(1)
			}
		}()
	}

	// Send commands to the workers
	for _, cmd := range cmds {
		cmdChan <- cmd
	}
	close(cmdChan)

	// Wait for all workers to finish
	wg.Wait()

	if svc.Import.NNDNs.Failed > 0 {
		log.Printf("🔥 WARNING 🔥 %d NNDNs failed to be created\n", svc.Import.NNDNs.Failed)
		for _, e := range svc.Import.Errors {
			log.Println(e)
		}
		return nil
	}

	log.Printf("✅ Created all NNDNs successfully\n")
	return nil
}

// creaetNNDN creaetes an NNDN through the API endpoint. If it already exists, that is not an error
func (svc *XMLEscrowService) createNNDN(client http.Client, cmd commands.CreateNNDNCommand) error {
	URL := BASE_URL + "/nndns"

	// UnMarshal the command into a JSON object
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	// Create the request
	req, err := http.NewRequest("POST", URL, bytes.NewReader(jsonCmd))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", BEARER)
	// Send it
	resp, err := client.Do(req)
	if err != nil {
		svc.Import.NNDNs.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error sending NNDN create request for %s: %s", cmd.Name, err.Error()))
		return err
	}

	// Fully read the body and close it - This is important for TCP connection reuse
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		svc.Import.NNDNs.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error reading response when creating NNDN with name %s", cmd.Name))
		return err
	}

	// Success
	if resp.StatusCode == 201 {
		svc.Import.NNDNs.Created++
		return nil
	}

	// unmarshall the response
	var response ErrorResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		svc.Import.NNDNs.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error unmarshalling NNDN response for %s: %s", cmd.Name, response.Error))
		return err
	}

	// Exists or Failed
	if resp.StatusCode == 400 {
		// if the contact already exists, we can skip it, we need to check the body for that

		// Exists, skip
		if strings.Contains(response.Error, "duplicate NNDN") {
			svc.Import.NNDNs.Existing++
			return nil
		}

		svc.Import.NNDNs.Failed++
		svc.Import.Errors = append(svc.Import.Errors, fmt.Sprintf("Error creating NNDN with name %s: %s", cmd.Name, response.Error))

	}

	return nil
}

// GetTLDFromAPI fetches the TLD from the API
func (svc *XMLEscrowService) GetTLDFromAPI(tldName string) (*entities.TLD, error) {
	URL := BASE_URL + "/tlds/" + tldName
	// Create a re-usable client
	client := getHTTPClient()

	// Create our TLD object
	tld := entities.TLD{}

	// Send the request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", BEARER)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Fully read the body and close it - This is important for TCP connection reuse
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Success
	if resp.StatusCode == 200 {
		err = json.Unmarshal(body, &tld)
		if err != nil {
			return nil, err
		}
		return &tld, nil
	}

	return nil, fmt.Errorf("error fetching TLD: %s", resp.Status)

}

// GetContactCountFromAPI fetches the object count from the API
func (svc *XMLEscrowService) GetContactCountFromAPI() (int64, error) {
	URL := BASE_URL + "/contacts/count"
	// Create a re-usable client
	client := getHTTPClient()

	var result *response.CountResult

	// Send the request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	// Fully read the body and close it - This is important for TCP connection reuse
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Success
	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, result)
		if err != nil {
			return 0, err
		}
		if result == nil {
			return 0, fmt.Errorf("error fetching contact count: response body is nil")
		}
		return result.Count, nil
	}

	return 0, fmt.Errorf("error fetching contact count: %s", resp.Status)
}

// getHTTPClient creates a re-usable http client optimized for tcp connections
func getHTTPClient() *http.Client {
	// Create a re-usable client optimized for tcp connections
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     1000,
	}
	return &http.Client{
		Transport: transport,
	}
}

// DuplicateHostCommands goes over the createHostCommands and checks if the clid of the domains the host is used on. If the host is used on domain(s) with different clid, the createHostCommand is duplicated with the other clid as a sponsor.
// This way we deal with deposits that have one host object owned by one clid but is used on different domains with different clids.
func (svc *XMLEscrowService) DuplicateHostCommands(cmds []commands.CreateHostCommand) ([]commands.CreateHostCommand, error) {
	// Load in the host-domain mapping from the -domainNameservers.csv file
	f, err := os.Open(svc.GetDepositFileNameWoExtension() + "-domainNameservers.csv")
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	// Create a map of hostnames to domains
	hostDomainMap := make(map[string][]string)
	for _, record := range records {
		host := strings.Trim(record[1], ".")
		domain := strings.Trim(record[0], ".")
		hostDomainMap[host] = append(hostDomainMap[host], domain)
	}
	// we're only interested in the hosts that are linked to multiple domains and the domains they are linked to
	// Actually that worked but left a small residue of hosts that are linked to only one domain, yet are not sponsored by the domain's clid
	// multiHostMap := make(map[string][]string)
	// interestingDomains := make(map[string]bool)
	// for k, v := range hostDomainMap {
	// 	if len(v) > 1 {
	// 		multiHostMap[k] = v
	// 		for _, domain := range v {
	// 			interestingDomains[domain] = true
	// 		}
	// 	}
	// }
	// fmt.Printf("Found %d hosts used on multiple domains\n", len(multiHostMap))
	// hostDomainMap = nil

	// Now read in the domains from the -domains.csv file
	f, err = os.Open(svc.GetDepositFileNameWoExtension() + "-domains.csv")
	if err != nil {
		return nil, err
	}
	r = csv.NewReader(f)
	records, err = r.ReadAll()
	if err != nil {
		return nil, err
	}
	// Create a map of domain names to clids for easy access
	domainClidMap := make(map[string]string)
	for _, record := range records {
		domain := strings.Trim(record[0], ".")
		clid := record[6]
		domainClidMap[domain] = clid
	}
	records = nil

	// Now loop over the hosts that appear on multiple domains and check if all the domains have the same clid as the host.
	// If clids don't match we need to create a host command with the other clid as the sponsor (and keeping all other data the same)
	// We only need to do this once per clid mismatch, so we can break out of the loop after the first mismatch created a new host command
	newCmds := []commands.CreateHostCommand{}
	for _, cmd := range cmds {
		if _, ok := hostDomainMap[cmd.Name]; !ok {
			// if the command is not about a host that we're interested in, continue
			continue
		}
		// Get the clid of the host
		hostClid := cmd.ClID

		// Get the clid of the domains the host is used on
		// create a list of clids for these domains
		domainClids := []string{}
		domains := hostDomainMap[cmd.Name]
		for _, domain := range domains {
			domainClids = append(domainClids, domainClidMap[domain])
		}

		// remove the duplicates from this list
		domainClids = removeDuplicates(domainClids)
		if len(domainClids) == 1 && domainClids[0] == hostClid.String() {
			// if all clids are the same, we can skip this host
			continue
		}

		// if we get here, we need to create a new host command for each clid that is different from the host clid
		for _, clid := range domainClids {
			if clid == hostClid.String() {
				continue
			}
			newCmd := cmd
			newCmd.ClID = entities.ClIDType(clid)
			if newCmd.CrRr != "" && newCmd.CrRr != hostClid {
				newCmd.CrRr = entities.ClIDType(clid)
			}
			if newCmd.UpRr != "" && newCmd.UpRr != hostClid {
				newCmd.UpRr = entities.ClIDType(clid)
			}
			newCmds = append(newCmds, newCmd)
		}

	}
	// Append the new commands to the old ones and return them
	cmds = append(cmds, newCmds...)
	return cmds, nil
}

// function to remove duplicate values
func removeDuplicates(s []string) []string {
	bucket := make(map[string]bool)
	var result []string
	for _, str := range s {
		if _, ok := bucket[str]; !ok {
			bucket[str] = true
			result = append(result, str)
		}
	}
	return result
}
