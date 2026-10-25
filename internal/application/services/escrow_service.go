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
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/schollz/progressbar/v3"
)

var (
	ErrDecodingToken = errors.New("error decoding token")
	ErrNoDepositTag  = errors.New("no deposit tag found")
	ErrNoHeaderTag   = errors.New("no header tag found")
	ErrDecodingXML   = errors.New("error decoding XML")
)

// XMLEscrowService implements XMLEscrowService interface
type XMLEscrowService struct {
	Deposit          entities.RDEDeposit             `json:"deposit"`
	Header           entities.RDEHeader              `json:"header"`
	Registrars       []entities.RDERegistrar         `json:"registrars"`
	IDNs             []entities.RDEIdnTableReference `json:"idns"`
	RegistrarMapping entities.RegistrarMapping       `json:"registrarMapping"`
	Analysis         entities.EscrowAnalysis         `json:"analysis"`
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
	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Analyzing deposit tag in %s (this may take a while for a large file) ... \n", svc.Deposit.FileName)
	for {
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
				return nil
			}
		}
	}
	return ErrNoDepositTag
}

// AnalyzeHeaderTag Analyzes the header tag
func (svc *XMLEscrowService) AnalyzeHeaderTag() error {
	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	log.Printf("Analyzing header tag in %s ... \n", svc.Deposit.FileName)
	for {
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
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Error parsing registrar entity for %s: %s", registrar.Name, err))
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

	// Create a writer for the createContactCommands file
	createContactsCommandsFile, err := os.Create(svc.GetDepositFileNameWoExtension() + "-createContactCommands.json")
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

					if cmd.Status.Linked && cmd.ID != "" {
						jsonCmd, err := json.Marshal(cmd)
						if err != nil {
							errCount++
							svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error creating JSON for contact command %s: %s", rdeContact.ID, err))
						}
						if _, werr := createContactsCommandsFile.Write(append(jsonCmd, '\n')); werr != nil {
							errCount++
							svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error writing contact command for %s: %s", rdeContact.ID, werr))
						}
					}

					// Write the contact to the contact file
					_ = contactWriter.Write(rdeContact.ToCSV()) // reported by Error() after Flush
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
						_ = statusWriter.Write([]string{rdeContact.ID, s}) // reported by Error() after Flush
					}
					// Set postalInfo in postalInfoFile
					cPostalInfo := make(map[int][]string)
					for i, postalInfo := range rdeContact.PostalInfo {
						postalInfoCounter++
						cPostalInfo[i] = append(cPostalInfo[i], rdeContact.ID)         // Add the contact ID as the first element
						cPostalInfo[i] = append(cPostalInfo[i], postalInfo.ToCSV()...) // Add the postal info
					}

					for _, v := range cPostalInfo {
						_ = postalInfoWriter.Write(v) // reported by Error() after Flush
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

				_ = pbar.Add(1) // progress display only
			}
		}
	}
	log.Println("Done!")
	if unlinkedCount > 0 {
		log.Printf("⚠️  WARNING %d unlinked contacts were found in the escrow file and will not be imported\n", unlinkedCount)
	}
	if postalInfoCounter < svc.Header.ContactCount()-unlinkedCount {
		log.Printf("⚠️  WARNING at least %d postalInfo objects, but found %d\n", svc.Header.ContactCount(), postalInfoCounter)
	}
	if statusCounter < svc.Header.ContactCount()-unlinkedCount {
		log.Printf("⚠️  WARNING Expected at least %d status objects, but found %d\n", svc.Header.ContactCount(), statusCounter)
	}
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	postalInfoWriter.Flush()
	checkLineCount(postalInfoFileName, postalInfoCounter)
	contactWriter.Flush()
	checkLineCount(outFileName, svc.Header.ContactCount()-unlinkedCount)
	for name, w := range map[string]*csv.Writer{"status": statusWriter, "postalInfo": postalInfoWriter, "contact": contactWriter} {
		if err := w.Error(); err != nil {
			return nil, fmt.Errorf("writing %s csv: %w", name, err)
		}
	}
	if errCount > 0 {
		log.Printf("🔥 ERROR %d errors were encountered while processing contacts. See analysis file for details\n", errCount)
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

	// Create a writer for the createHostCommands file
	createHostCommandsFile, err := os.Create(svc.GetDepositFileNameWoExtension() + "-createHostCommands.json")
	if err != nil {
		return nil, err
	}

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

				_ = writer.Write(host.ToCSV()) // reported by Error() after Flush
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
					_ = statusWriter.Write([]string{host.Name, s}) // reported by Error() after Flush
				}
				// Set addresses in addrFile
				for _, addr := range host.Addr {
					addrCounter++
					_ = addrWriter.Write([]string{host.Name, addr.IP, addr.ID}) // reported by Error() after Flush
				}

				// Add the command to our slice of create commands
				if returnHostCommands {
					// Unset the linked status in case the deposit claims its linked but there is no domain linked to it.
					// Once we linke the hosts and domains the status will be updated
					cmd.Status.Linked = false
					hostCmds = append(hostCmds, cmd)
				}

				// Write them to file
				jsonCmd, err := json.Marshal(cmd)
				if err != nil {
					errCount++
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Error creating JSON for host command %s: %s", host.Name, err))
				}
				if _, werr := createHostCommandsFile.Write(append(jsonCmd, '\n')); werr != nil {
					errCount++
					svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error writing host command for %s: %s", host.Name, werr))
				}

				// Update counters in Registrar Map
				objCount := svc.RegistrarMapping[host.ClID]
				objCount.HostCount++
				svc.RegistrarMapping[host.ClID] = objCount
				count++

				_ = pbar.Add(1) // progress display only
			}
		}
	}
	log.Println("Done!")
	if unlinkedCount > svc.Header.HostCount()/10 {
		log.Printf("⚠️  WARNING %d more than 1/10th hosts are unlinked, but can still be imported\n", unlinkedCount)
	}
	addrWriter.Flush()
	checkLineCount(addrFileName, addrCounter)
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	writer.Flush()
	checkLineCount(outFileName, svc.Header.HostCount())
	for name, w := range map[string]*csv.Writer{"hostAddress": addrWriter, "hostStatus": statusWriter, "host": writer} {
		if err := w.Error(); err != nil {
			return nil, fmt.Errorf("writing %s csv: %w", name, err)
		}
	}
	if errCount > 0 {
		log.Printf("🔥 ERROR %d errors were encountered while processing hosts. See analysis file for details\n", errCount)
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

	// Create a writer for the createNNDNCommands file
	createNNDNCommandsFile, err := os.Create(svc.GetDepositFileNameWoExtension() + "-createNNDNCommands.json")
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

				// Write them to file
				jsonCmd, err := json.Marshal(cmd)
				if err != nil {
					errCount++
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, fmt.Sprintf("Error creating JSON for NNDN command %s: %s", rdeNNDN.AName, err))
				}
				if _, werr := createNNDNCommandsFile.Write(append(jsonCmd, '\n')); werr != nil {
					errCount++
					svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error writing NNDN command for %s: %s", rdeNNDN.AName, werr))
				}

				// Write to CSV
				_ = writer.Write(rdeNNDN.ToCSV()) // reported by Error() after Flush
				count++

				_ = pbar.Add(1) // progress display only
			}
		}
	}
	writer.Flush()
	checkLineCount(outFileName, svc.Header.NNDNCount())
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("writing nndn csv: %w", err)
	}
	if errCount > 0 {
		log.Printf("🔥 ERROR %d errors were encountered while processing NNDNs. See analysis file for details\n", errCount)
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
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		log.Printf("⚠️  WARNING could not open %s to verify its line count: %v\n", filename, err)
		return
	}
	defer file.Close()
	lineCount, err := CountLines(file)
	if err != nil {
		log.Printf("⚠️  WARNING could not count lines in %s: %v\n", filename, err)
		return
	}
	var tip = ""
	if lineCount != expected {
		if lineCount > expected {
			tip = `This might indicate there are newline(\n) characters in the data.`
		}
		log.Printf("⚠️  WARNING Expecting %d objects, found %d objects in %s %s\n", expected, lineCount, filename, tip)
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

	// Create a writer for the createDomainCommands file
	createDomainCommandsFileName := svc.GetDepositFileNameWoExtension() + "-createDomainCommands.json"
	createDomainCommandsFile, err := os.Create(createDomainCommandsFileName)
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
				result, err := cmd.FromRdeDomain(&dom)
				if err != nil {
					errCount++
					svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("error creating domain command for %s: %s", dom.Name, err))
				}
				if err == nil && result == nil {
					errCount++
					svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("received no error and a nil create command for %s", dom.Name))
				}

				if result != nil && result.Warnings != nil {
					svc.Analysis.Warnings = append(svc.Analysis.Warnings, result.Warnings...)
				}

				if returnCommands {
					domCreateCommands = append(domCreateCommands, cmd)
				}

				// Write the create command to file if it's not nil
				if result != nil {
					jsonCmd, err := json.Marshal(cmd)
					if err != nil {
						errCount++
						svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("error marshalling domain command for %s: %s", dom.Name, err))
					}
					if _, werr := createDomainCommandsFile.Write(append(jsonCmd, '\n')); werr != nil {
						errCount++
						svc.Analysis.Errors = append(svc.Analysis.Errors, fmt.Sprintf("Error writing domain command for %s: %s", dom.Name.String(), werr))
					}
				}

				// Write the domain to the domain file
				_ = domainWriter.Write(dom.ToCSV()) // reported by Error() after Flush
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
					_ = statusWriter.Write([]string{dom.Name.String(), s}) // reported by Error() after Flush
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
					_ = nameserverWriter.Write([]string{dom.Name.String(), ns}) // reported by Error() after Flush
				}
				// Write the dnssec information to the dnssec file
				for _, dsData := range dom.SecDNS.DSData {
					dnssecCounter++
					_ = dnssecWriter.Write([]string{dom.Name.String(), strconv.Itoa(dsData.KeyTag), strconv.Itoa(dsData.Alg), strconv.Itoa(dsData.DigestType), dsData.Digest}) // reported by Error() after Flush
				}
				// Write the transfer information to the transfer file
				if dom.TrnData.TrStatus.State != "" {
					transferCounter++
					_ = transferWriter.Write([]string{dom.Name.String(), dom.TrnData.TrStatus.State, dom.TrnData.ReRr.RegID, dom.TrnData.ReDate, dom.TrnData.ReRr.RegID, dom.TrnData.AcDate, dom.TrnData.ExDate}) // reported by Error() after Flush
				}

				// Update counters in Registrar Map
				objCount := svc.RegistrarMapping[dom.ClID]
				objCount.DomainCount++
				svc.RegistrarMapping[dom.ClID] = objCount
				count++

				_ = pbar.Add(1) // progress display only
			}
		}
	}
	// Write the unique contact IDs to the contactID file
	for k := range svc.uniqueContactIDs {
		_ = contactIDWriter.Write([]string{k}) // reported by Error() after Flush
	}
	contactIDWriter.Flush()
	log.Printf("✅  Written %d unique contact IDs used by Domains to : %s", len(svc.uniqueContactIDs), contactIDFileName)
	log.Println("Done!")
	if statusCounter < svc.Header.DomainCount() {
		log.Printf("⚠️  WARNING Expected at least %d status objects, but found %d\n", svc.Header.DomainCount(), statusCounter)
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
	for name, w := range map[string]*csv.Writer{"contactID": contactIDWriter, "domainStatus": statusWriter, "domainNameserver": nameserverWriter, "domainDnssec": dnssecWriter, "domainTransfer": transferWriter, "domain": domainWriter} {
		if err := w.Error(); err != nil {
			return nil, fmt.Errorf("writing %s csv: %w", name, err)
		}
	}
	if errCount > 0 {
		log.Printf("🔥 ERROR %d errors were encountered while processing domains. See analysis file for details\n", errCount)
	}
	return domCreateCommands, nil
}

// Count the number of lines in a file by looking for \n occurrences. Use this to check against the number of objects in the header
func CountLines(r io.Reader) (int, error) {
	var count int
	var read int
	var err error
	target := []byte("\n")

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

// getUniqueContactIDs streams through the contacts CSV row-by-row and
// collects column-0 (contact ID) into a set. Only the ID strings are
// retained — the full CSV rows are never held in memory.
func (svc *XMLEscrowService) getUniqueContactIDs() (map[string]bool, error) {
	contactIDs := make(map[string]bool)
	f, err := os.Open(svc.GetDepositFileNameWoExtension() + "-contacts.csv")
	if err != nil {
		return contactIDs, err
	}
	defer f.Close()
	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return contactIDs, err
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return contactIDs, err
		}
		if len(record) > 0 {
			contactIDs[record[0]] = true
		}
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

// LookForMissingContacts checks whether every contact ID referenced by
// domains (in -uniqueDomainContactIDs.csv) actually exists in the
// contacts file. Both CSVs are streamed row-by-row — no ReadAll().
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

	// Skip header
	if _, err := r.Read(); err != nil {
		return err
	}

	errorCount := 0
	missingContactIDs := []string{}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) > 0 && !svc.uniqueContactIDs[record[0]] {
			errorCount++
			missingContactIDs = append(missingContactIDs, record[0])
		}
	}
	if errorCount > 0 {
		svc.Analysis.MissingContacts = missingContactIDs
		log.Printf("🔥 ERROR Found %d missing contact IDs in %s \n", errorCount, svc.GetDepositFileNameWoExtension()+"-uniqueDomainContactIDs.csv")
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
	if err := os.WriteFile(analysisFileName, bytes, 0600); err != nil {
		return fmt.Errorf("saving analysis to %s: %w", analysisFileName, err)
	}
	log.Printf("✅  Saved analysis to: %s\n", analysisFileName)
	return nil
}
