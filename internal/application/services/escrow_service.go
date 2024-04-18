package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
	Deposit           entities.RDEDeposit             `json:"deposit"`
	Header            entities.RDEHeader              `json:"header"`
	Registrars        []entities.RDERegistrar         `json:"registrars"`
	IDNs              []entities.RDEIdnTableReference `json:"idns"`
	RegsistrarMapping entities.RegsitrarMapping       `json:"registrarMapping"`
	Analysis          entities.EscrowAnalysis         `json:"analysis"`
}

// NewXMLEscrowService creates a new instance of EscrowService
func NewXMLEscrowService(XMLFilename string) (*XMLEscrowService, error) {
	// Fail fast if we can't open the file
	f, _ := os.Open(XMLFilename)
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
	d.RegsistrarMapping = entities.NewRegistrarMapping()

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
					log.Printf("Error parsing registrar %s: %s\n", registrar.Name, err)
					log.Printf("Registrar.PostalInfo: %v\n", registrar.PostalInfo)
					panic(err)
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
func (svc *XMLEscrowService) ExtractContacts() error {

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

	log.Printf("Looking up %d contacts... \n", svc.Header.ContactCount())
	pbar := progressbar.Default(int64(svc.Header.ContactCount()))

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
			return errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type contact
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "contact" {
				// Skip contacts that are not in the contact namespace
				if se.Name.Space != entities.CONTACT_URI {
					continue
				}
				var contact entities.RDEContact
				if err := d.DecodeElement(&contact, &se); err != nil {
					return errors.Join(ErrDecodingXML, err)
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

				pbar.Add(1)
			}
		}
	}
	log.Println("Done!")
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

// ExtractHosts Extracts hosts from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-hosts.csv
// - {inputFilename}-hostStatuses.csv
// - {inputFilename}-hostAddresses.csv
func (svc *XMLEscrowService) ExtractHosts() error {

	count := 0

	f, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return err
	}
	defer f.Close()

	d := xml.NewDecoder(f)

	// Prepare the CSV file to receive the hosts
	outFileName := svc.GetDepositFileNameWoExtension() + "-hosts.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	writer := csv.NewWriter(outFile)

	// Prepare the CSV file to receive the host statuses
	statusFileName := svc.GetDepositFileNameWoExtension() + "-hostStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return err
	}
	defer statusFile.Close()
	statusWriter := csv.NewWriter(statusFile)

	// Prepare the CSV file to receive the host addresses
	addrFileName := svc.GetDepositFileNameWoExtension() + "-hostAddresses.csv"
	addrFile, err := os.Create(addrFileName)
	if err != nil {
		return err
	}
	defer addrFile.Close()
	addrWriter := csv.NewWriter(addrFile)
	addrCounter := 0
	statusCounter := 0

	log.Printf("Looking up %d hosts... \n", svc.Header.HostCount())
	pbar := progressbar.Default(int64(svc.Header.HostCount()))

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
			return errors.Join(ErrDecodingToken, tokenErr)
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
					return errors.Join(ErrDecodingXML, err)
				}
				writer.Write([]string{host.Name, host.RoID, host.ClID, host.CrRr, host.CrDate, host.UpRr, host.UpDate})
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

				// Update counters in Registrar Map
				objCount := svc.RegsistrarMapping[host.ClID]
				objCount.HostCount++
				svc.RegsistrarMapping[host.ClID] = objCount
				count++

				pbar.Add(1)
			}
		}
	}
	log.Println("Done!")
	addrWriter.Flush()
	checkLineCount(addrFileName, addrCounter)
	statusWriter.Flush()
	checkLineCount(statusFileName, statusCounter)
	writer.Flush()
	checkLineCount(outFileName, svc.Header.HostCount())
	return nil
}

// ExtractNNDNS Extracts statuses from the escrow file and writes them to a CSV file
// This will output the following files:
//
// - {inputFilename}-nndns.csv
func (svc *XMLEscrowService) ExtractNNDNS() error {

	count := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	// Prepare the CSV file to receive the nndns
	outFileName := svc.GetDepositFileNameWoExtension() + "-nndns.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)

	log.Printf("Looking up %d nndns... \n", svc.Header.NNDNCount())
	pbar := progressbar.Default(int64(svc.Header.NNDNCount()))

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
			return errors.Join(ErrDecodingToken, tokenErr)
		}
		// Only process start elements of type nndn
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "NNDN" {
				// Skip nndns that are not in the nndns namespace
				if se.Name.Space != entities.NNDN_URI {
					continue
				}
				var nndns entities.RDENNDN
				if err := d.DecodeElement(&nndns, &se); err != nil {
					return errors.Join(ErrDecodingXML, err)
				}
				writer.Write([]string{nndns.AName, nndns.UName, nndns.IDNTableID, nndns.OriginalName, nndns.NameState, nndns.CrDate})
				count++

				pbar.Add(1)
			}
		}
	}
	writer.Flush()
	checkLineCount(outFileName, svc.Header.NNDNCount())
	return nil
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
	if lineCount != expected {
		log.Printf("🔥 WARNING 🔥 Expecting %d objects, found %d objects in %s \n", expected, lineCount, filename)
		if lineCount > expected {
			log.Println(`This might indicate there are newline(\n) characters in the data.`)
		}
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
func (svc *XMLEscrowService) ExtractDomains() error {

	count := 0

	d, err := svc.getXMLDecoder()
	if err != nil {
		return err
	}

	// Create a CSV file and writer to write the main domain information to
	outFileName := svc.GetDepositFileNameWoExtension() + "-domains.csv"
	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	domainWriter := csv.NewWriter(outFile)

	// Create a -status CSV file and writer to write the domain statuses to
	statusFileName := svc.GetDepositFileNameWoExtension() + "-domainStatuses.csv"
	statusFile, err := os.Create(statusFileName)
	if err != nil {
		return err
	}
	statusWriter := csv.NewWriter(statusFile)
	statusCounter := 0

	// Create a -nameserver CSV file and writer to write the nameservers to
	nameserverFileName := svc.GetDepositFileNameWoExtension() + "-domainNameservers.csv"
	nameserverFile, err := os.Create(nameserverFileName)
	if err != nil {
		return err
	}
	nameserverWriter := csv.NewWriter(nameserverFile)
	nameServerCounter := 0

	// Create a -dnssec CSV file and writer to write the dnssec information to
	dnssecFileName := svc.GetDepositFileNameWoExtension() + "-DomainDnssec.csv"
	dnssecFile, err := os.Create(dnssecFileName)
	if err != nil {
		return err
	}
	dnssecWriter := csv.NewWriter(dnssecFile)
	dnssecCounter := 0

	// Create a -transfers CSV file and writer to write the transfer information to
	transferFileName := svc.GetDepositFileNameWoExtension() + "-domainTransfers.csv"
	transferFile, err := os.Create(transferFileName)
	if err != nil {
		return err
	}
	transferWriter := csv.NewWriter(transferFile)
	transferCounter := 0

	// Create a file and writer to write the unique contact IDs to
	contactIDFileName := svc.GetDepositFileNameWoExtension() + "-uniqueDomainContactIDs.csv"
	contactIDFile, err := os.Create(contactIDFileName)
	if err != nil {
		return err
	}
	contactIDWriter := csv.NewWriter(contactIDFile)
	uniqueContactIDs := make(map[string]bool)

	log.Printf("Looking up %d domains... \n", svc.Header.DomainCount())
	pbar := progressbar.Default(int64(svc.Header.DomainCount()))

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
			return errors.Join(ErrDecodingToken, tokenErr)
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
					return errors.Join(ErrDecodingXML, err)
				}
				// Write the domain to the domain file
				domainWriter.Write([]string{string(dom.Name), dom.RoID, dom.UName, dom.IdnTableId, dom.OriginalName, dom.Registrant, dom.ClID, dom.CrRr, dom.CrDate, dom.ExDate, dom.UpRr, dom.UpDate})
				// Add a line to the contactID file for each contact, only if it does not exist yet
				for _, contact := range dom.Contact {
					// Only add it if it is not there already
					if !uniqueContactIDs[contact.ID] {
						uniqueContactIDs[contact.ID] = true
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
				objCount := svc.RegsistrarMapping[dom.ClID]
				objCount.DomainCount++
				svc.RegsistrarMapping[dom.ClID] = objCount
				count++

				pbar.Add(1)
			}
		}
	}
	// Write the unique contact IDs to the contactID file
	for k := range uniqueContactIDs {
		contactIDWriter.Write([]string{k})
	}
	contactIDWriter.Flush()
	log.Printf("✅  Written %d unique contact IDs used by Domains to : %s", len(uniqueContactIDs), contactIDFileName)
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
	return nil
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

// LookForMissingContacts Looks if all the uniqueContactIDs used on domains are present in the contact file. It saves the results in the escrow object
func (svc *XMLEscrowService) LookForMissingContacts() error {
	contactIDs, err := svc.getUniqueContactIDs()
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
		if !contactIDs[record[0]] {
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

	// BASE_URL := "http://domain-os-admin-api-1:8080"
	BASE_URL := "http://localhost:8080"
	bearer := "Bearer " + os.Getenv("EPP_API_TOKEN")

	var found = 0
	var missing = 0
	var missingGurIDs = []int{}

	for _, rar := range svc.Registrars {

		var URL string

		// Handle special cases of reserved GurIDs
		if rar.GurID == 9997 {
			URL = BASE_URL + "/registrars/9997-ICANN-SLAM"
		} else if rar.GurID == 9999 || rar.GurID == 119 {
			URL = BASE_URL + "/registrars/9999" + "-" + svc.Header.TLD
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
			if svc.RegsistrarMapping[rar.ID].DomainCount == 0 {
				log.Printf("Registrar %s with GurID %d not found, but has no domains, skipping ...", rar.Name, rar.GurID)
				continue
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
			rarMap := svc.RegsistrarMapping[rar.ID]
			rarMap.Name = rar.Name
			rarMap.GurID = rar.GurID
			rarMap.RegistrarClID = responseRar.ClID
			svc.RegsistrarMapping[rar.ID] = rarMap
			found++
			continue
		}

		// other error
		log.Printf("got a %s: %s", resp.Status, URL)
		missing++
		missingGurIDs = append(missingGurIDs, rar.GurID)

	}
	// write mapping to file
	for k, v := range svc.RegsistrarMapping {
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
