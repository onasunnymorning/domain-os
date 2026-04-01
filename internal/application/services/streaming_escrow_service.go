package services

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// StreamingCSVWriters holds all CSV writers for concurrent writing during streaming
type StreamingCSVWriters struct {
	// Domain-related files
	domainFile             *os.File
	domainWriter           *csv.Writer
	domainStatusFile       *os.File
	domainStatusWriter     *csv.Writer
	domainNameserverFile   *os.File
	domainNameserverWriter *csv.Writer
	domainTransferFile     *os.File
	domainTransferWriter   *csv.Writer
	domainDnssecFile       *os.File
	domainDnssecWriter     *csv.Writer
	domainRgpStatusFile    *os.File
	domainRgpStatusWriter  *csv.Writer

	// Contact-related files
	contactFile             *os.File
	contactWriter           *csv.Writer
	contactStatusFile       *os.File
	contactStatusWriter     *csv.Writer
	contactPostalInfoFile   *os.File
	contactPostalInfoWriter *csv.Writer

	// Host-related files
	hostFile          *os.File
	hostWriter        *csv.Writer
	hostStatusFile    *os.File
	hostStatusWriter  *csv.Writer
	hostAddressFile   *os.File
	hostAddressWriter *csv.Writer

	// Registrar-related files
	registrarFile             *os.File
	registrarWriter           *csv.Writer
	registrarPostalInfoFile   *os.File
	registrarPostalInfoWriter *csv.Writer

	// Other files
	nndnFile              *os.File
	nndnWriter            *csv.Writer
	uniqueContactIDFile   *os.File
	uniqueContactIDWriter *csv.Writer

	// Counters
	domainStatusCounter      int
	domainNameserverCounter  int
	domainTransferCounter    int
	domainDnssecCounter      int
	domainRgpStatusCounter   int
	contactStatusCounter     int
	contactPostalInfoCounter int
	hostStatusCounter        int
	hostAddressCounter       int
}

// StreamingXMLEscrowService provides single-pass XML streaming analysis
type StreamingXMLEscrowService struct {
	*XMLEscrowService
	// CSV writers for streaming output
	csvWriters *StreamingCSVWriters
}

// NewStreamingXMLEscrowService creates a new streaming service wrapper
func NewStreamingXMLEscrowService(xmlFilename string) (*StreamingXMLEscrowService, error) {
	baseService, err := NewXMLEscrowService(xmlFilename)
	if err != nil {
		return nil, err
	}

	service := &StreamingXMLEscrowService{
		XMLEscrowService: baseService,
	}

	// Initialize CSV writers
	if err := service.initializeCSVWriters(); err != nil {
		return nil, err
	}

	return service, nil
}

// HeartbeatFunc is a callback to report progress/liveness
type HeartbeatFunc func(details ...interface{})

// StreamAnalyze performs single-pass analysis of the XML file
func (svc *StreamingXMLEscrowService) StreamAnalyze(mapRegistrars bool, token string, heartbeat HeartbeatFunc) error {

	// Perform single-pass streaming
	if err := svc.streamXML(heartbeat); err != nil {
		return err
	}

	// Post-processing steps
	svc.UnlinkedContactCheck()
	if err := svc.LookForMissingContacts(); err != nil {
		return err
	}

	// MapRegistrars logic has been moved to a separate activity (MapRegistrars)
	// We no longer call it here inline.
	// But we keep the argument for backward compatibility or if we want to restore it later
	// (though the signature of MapRegistrars changed to require overrides, so we can't call it easily without them)
	if mapRegistrars {
		log.Println("⚠️ MapRegistrars argument is true, but inline mapping is deprecated/disabled in favor of MapRegistrars activity.")
	}

	// Save analysis results
	if err := svc.SaveAnalysis(); err != nil {
		return err
	}

	log.Println("Streaming analysis completed successfully")
	return nil
}

// Removed complex handler system in favor of simple direct processing

// streamXML performs the single-pass streaming through the XML file
func (svc *StreamingXMLEscrowService) streamXML(heartbeat HeartbeatFunc) error {
	file, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)

	log.Printf("Streaming through %s...\n", svc.Deposit.FileName)

	// Simple counters for each tag type
	var depositFound, headerFound bool
	var domainCount, contactCount, hostCount, registrarCount, idnTableRefCount, nndnCount int

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Join(ErrDecodingToken, err)
		}

		switch startElement := token.(type) {
		case xml.StartElement:
			tagName := startElement.Name.Local
			namespace := startElement.Name.Space

			// Skip debug for common XML structure elements to reduce noise

			// Route to specific processing functions based on tag name and namespace
			switch tagName {
			case "deposit":
				if !depositFound {
					if err := svc.processDepositTag(decoder, startElement); err != nil {
						log.Printf("Error processing deposit tag: %v", err)
					}
					depositFound = true
				}

			case "header":
				if !headerFound {
					if err := svc.processHeaderTag(decoder, startElement); err != nil {
						log.Printf("Error processing header tag: %v", err)
					} else {
						headerFound = true
						log.Printf("✅ Processed header tag - Domains: %d, Contacts: %d, Hosts: %d",
							svc.Header.DomainCount(),
							svc.Header.ContactCount(),
							svc.Header.HostCount())
					}
				}

			case "domain":
				if namespace == entities.DOMAIN_URI {
					if err := svc.processDomainTag(decoder, startElement); err != nil {
						log.Printf("Error processing domain tag: %v", err)
					} else {
						domainCount++
						// Log progress every 100,000 domains
						if domainCount%100000 == 0 {
							log.Printf("Processed %d domains so far...", domainCount)
						}
						// Heartbeat every 1,000 domains to keep Temporal happy
						if domainCount%1000 == 0 && heartbeat != nil {
							heartbeat("processing domains", domainCount)
						}
					}
				}

			case "contact":
				if namespace == entities.CONTACT_URI {
					if err := svc.processContactTag(decoder, startElement); err != nil {
						log.Printf("Error processing contact tag: %v", err)
					} else {
						contactCount++
						// Log progress every 100,000 contacts
						if contactCount%100000 == 0 {
							log.Printf("Processed %d contacts so far...", contactCount)
						}
						// Heartbeat every 1,000 contacts
						if contactCount%1000 == 0 && heartbeat != nil {
							heartbeat("processing contacts", contactCount)
						}
					}
				}

			case "host":
				if namespace == entities.HOST_URI {
					if err := svc.processHostTag(decoder, startElement); err != nil {
						log.Printf("Error processing host tag: %v", err)
					} else {
						hostCount++
					}
				}

			case "registrar":
				if namespace == entities.REGISTRAR_URI {
					if err := svc.processRegistrarTag(decoder, startElement); err != nil {
						log.Printf("Error processing registrar tag: %v", err)
					} else {
						registrarCount++
					}
				}

			case "idnTableRef":
				if namespace == entities.IDN_URI {
					if err := svc.processIDNTableRefTag(decoder, startElement); err != nil {
						log.Printf("Error processing IDN table ref tag: %v", err)
					} else {
						idnTableRefCount++
					}
				}

			case "NNDN": // Note: NNDN elements are uppercase, not lowercase
				if namespace == entities.NNDN_URI {
					if err := svc.processNNDNTag(decoder, startElement); err != nil {
						log.Printf("Error processing NNDN tag: %v", err)
					} else {
						nndnCount++
						if nndnCount == 1 {
							log.Printf("✅ Started processing NNDN elements")
						}
					}
				} else {
					log.Printf("⚠️  Found NNDN tag with unexpected namespace: '%s' (expected: '%s')", namespace, entities.NNDN_URI)
				}

			case "watermark":
				// Process watermark element
				var watermark string
				if err := decoder.DecodeElement(&watermark, &startElement); err != nil {
					log.Printf("Error processing watermark: %v", err)
				} else {
					svc.Deposit.Watermark = watermark
					log.Printf("✅ Found watermark: %s", watermark)
				}
			}
		}
	}

	// Log final counts
	log.Printf("✅ Processed %d domain tags", domainCount)
	log.Printf("✅ Processed %d contact tags", contactCount)
	log.Printf("✅ Processed %d host tags", hostCount)
	log.Printf("✅ Processed %d registrar tags", registrarCount)
	log.Printf("✅ Processed %d IDN table reference tags", idnTableRefCount)
	log.Printf("✅ Processed %d NNDN tags", nndnCount)

	// Validate counts against header expectations
	if headerFound {
		expectedDomains := svc.Header.DomainCount()
		expectedContacts := svc.Header.ContactCount()
		expectedHosts := svc.Header.HostCount()
		expectedRegistrars := svc.Header.RegistrarCount()
		expectedNNDNs := svc.Header.NNDNCount()

		if domainCount != expectedDomains {
			log.Printf("⚠️  Domain count mismatch: processed %d, expected %d", domainCount, expectedDomains)
		}
		if contactCount != expectedContacts {
			log.Printf("⚠️  Contact count mismatch: processed %d, expected %d", contactCount, expectedContacts)
		}
		if hostCount != expectedHosts {
			log.Printf("⚠️  Host count mismatch: processed %d, expected %d", hostCount, expectedHosts)
		}
		if registrarCount != expectedRegistrars {
			log.Printf("⚠️  Registrar count mismatch: processed %d, expected %d", registrarCount, expectedRegistrars)
		}
		if nndnCount != expectedNNDNs {
			log.Printf("⚠️  NNDN count mismatch: processed %d, expected %d", nndnCount, expectedNNDNs)
		}
	}

	// Close and flush all CSV files
	if err := svc.closeCSVWriters(); err != nil {
		return err
	}

	// Validate CSV line counts (accounting for headers)
	if headerFound {
		if err := svc.validateCSVCounts(); err != nil {
			log.Printf("⚠️  CSV validation error: %v", err)
		}
	}

	return nil
}

// initializeCSVWriters creates and opens all CSV files for streaming output
func (svc *StreamingXMLEscrowService) initializeCSVWriters() error {
	baseFilename := svc.GetDepositFileNameWoExtension()
	writers := &StreamingCSVWriters{}

	// Initialize domain-related CSV files
	var err error
	if writers.domainFile, err = os.Create(baseFilename + "-domains.csv"); err != nil {
		return err
	}
	writers.domainWriter = csv.NewWriter(writers.domainFile)
	writers.domainWriter.Write(entities.RdeDomainCSVHeader) // Write header

	if writers.domainStatusFile, err = os.Create(baseFilename + "-domainStatuses.csv"); err != nil {
		return err
	}
	writers.domainStatusWriter = csv.NewWriter(writers.domainStatusFile)
	writers.domainStatusWriter.Write([]string{"DomainName", "Status"}) // Write header

	if writers.domainNameserverFile, err = os.Create(baseFilename + "-domainNameservers.csv"); err != nil {
		return err
	}
	writers.domainNameserverWriter = csv.NewWriter(writers.domainNameserverFile)
	writers.domainNameserverWriter.Write([]string{"DomainName", "Nameserver"}) // Write header

	if writers.domainTransferFile, err = os.Create(baseFilename + "-domainTransfers.csv"); err != nil {
		return err
	}
	writers.domainTransferWriter = csv.NewWriter(writers.domainTransferFile)
	writers.domainTransferWriter.Write([]string{"DomainName", "TransferStatus", "RelinquishingRegistrar", "RelinquishDate", "AcquiringRegistrar", "AcquireDate", "ExpiryDate"}) // Write header

	if writers.domainDnssecFile, err = os.Create(baseFilename + "-DomainDnssec.csv"); err != nil {
		return err
	}
	writers.domainDnssecWriter = csv.NewWriter(writers.domainDnssecFile)
	writers.domainDnssecWriter.Write([]string{"DomainName", "DnssecData"}) // Write header

	if writers.domainRgpStatusFile, err = os.Create(baseFilename + "-domainRgpStatus.csv"); err != nil {
		return err
	}
	writers.domainRgpStatusWriter = csv.NewWriter(writers.domainRgpStatusFile)
	writers.domainRgpStatusWriter.Write([]string{"DomainName", "RgpStatus"}) // Write header

	// Initialize contact-related CSV files
	if writers.contactFile, err = os.Create(baseFilename + "-contacts.csv"); err != nil {
		return err
	}
	writers.contactWriter = csv.NewWriter(writers.contactFile)
	writers.contactWriter.Write(entities.RdeContactCSVHeader) // Write header

	if writers.contactStatusFile, err = os.Create(baseFilename + "-contactStatuses.csv"); err != nil {
		return err
	}
	writers.contactStatusWriter = csv.NewWriter(writers.contactStatusFile)
	writers.contactStatusWriter.Write([]string{"ContactID", "Status"}) // Write header

	if writers.contactPostalInfoFile, err = os.Create(baseFilename + "-contactPostalInfo.csv"); err != nil {
		return err
	}
	writers.contactPostalInfoWriter = csv.NewWriter(writers.contactPostalInfoFile)
	// ContactID + postal info fields
	contactPostalHeader := append([]string{"ContactID"}, entities.RdeContactPostalInfoCSVHeader...)
	writers.contactPostalInfoWriter.Write(contactPostalHeader) // Write header

	// Initialize host-related CSV files
	if writers.hostFile, err = os.Create(baseFilename + "-hosts.csv"); err != nil {
		return err
	}
	writers.hostWriter = csv.NewWriter(writers.hostFile)
	writers.hostWriter.Write(entities.RdeHostCSVHeader) // Write header

	if writers.hostStatusFile, err = os.Create(baseFilename + "-hostStatuses.csv"); err != nil {
		return err
	}
	writers.hostStatusWriter = csv.NewWriter(writers.hostStatusFile)
	writers.hostStatusWriter.Write([]string{"HostName", "Status"}) // Write header

	if writers.hostAddressFile, err = os.Create(baseFilename + "-hostAddresses.csv"); err != nil {
		return err
	}
	writers.hostAddressWriter = csv.NewWriter(writers.hostAddressFile)
	writers.hostAddressWriter.Write([]string{"HostName", "IPAddress", "IPVersion"}) // Write header

	// Initialize registrar-related CSV files
	if writers.registrarFile, err = os.Create(baseFilename + "-registrars.csv"); err != nil {
		return err
	}
	writers.registrarWriter = csv.NewWriter(writers.registrarFile)
	writers.registrarWriter.Write([]string{"ID", "Name", "GurID", "Status", "Voice", "Fax", "Email", "URL", "CrDate", "UpDate"}) // Write header

	if writers.registrarPostalInfoFile, err = os.Create(baseFilename + "-registrarPostalInfo.csv"); err != nil {
		return err
	}
	writers.registrarPostalInfoWriter = csv.NewWriter(writers.registrarPostalInfoFile)
	writers.registrarPostalInfoWriter.Write([]string{"RegistrarID", "Type", "Street1", "Street2", "Street3", "City", "StateProvince", "PostalCode", "CountryCode"}) // Write header

	// Initialize other CSV files
	if writers.nndnFile, err = os.Create(baseFilename + "-nndns.csv"); err != nil {
		return err
	}
	writers.nndnWriter = csv.NewWriter(writers.nndnFile)
	writers.nndnWriter.Write(entities.RdeNNDNCSVHeader) // Write header

	if writers.uniqueContactIDFile, err = os.Create(baseFilename + "-uniqueDomainContactIDs.csv"); err != nil {
		return err
	}
	writers.uniqueContactIDWriter = csv.NewWriter(writers.uniqueContactIDFile)
	writers.uniqueContactIDWriter.Write([]string{"ContactID"}) // Write header

	svc.csvWriters = writers
	log.Printf("✅ Initialized all CSV files for streaming output")
	return nil
}

// closeCSVWriters flushes and closes all CSV files
func (svc *StreamingXMLEscrowService) closeCSVWriters() error {
	if svc.csvWriters == nil {
		return nil
	}

	writers := svc.csvWriters

	// Flush and close all writers
	writers.domainWriter.Flush()
	writers.domainFile.Close()
	writers.domainStatusWriter.Flush()
	writers.domainStatusFile.Close()
	writers.domainNameserverWriter.Flush()
	writers.domainNameserverFile.Close()
	writers.domainTransferWriter.Flush()
	writers.domainTransferFile.Close()
	writers.domainDnssecWriter.Flush()
	writers.domainDnssecFile.Close()
	writers.domainRgpStatusWriter.Flush()
	writers.domainRgpStatusFile.Close()

	writers.contactWriter.Flush()
	writers.contactFile.Close()
	writers.contactStatusWriter.Flush()
	writers.contactStatusFile.Close()
	writers.contactPostalInfoWriter.Flush()
	writers.contactPostalInfoFile.Close()

	writers.hostWriter.Flush()
	writers.hostFile.Close()
	writers.hostStatusWriter.Flush()
	writers.hostStatusFile.Close()
	writers.hostAddressWriter.Flush()
	writers.hostAddressFile.Close()

	writers.registrarWriter.Flush()
	writers.registrarFile.Close()
	writers.registrarPostalInfoWriter.Flush()
	writers.registrarPostalInfoFile.Close()

	writers.nndnWriter.Flush()
	writers.nndnFile.Close()

	// Write unique contact IDs to file
	for contactID := range svc.uniqueContactIDs {
		writers.uniqueContactIDWriter.Write([]string{contactID})
	}
	writers.uniqueContactIDWriter.Flush()
	writers.uniqueContactIDFile.Close()

	log.Printf("✅ Closed all CSV files and wrote %d unique contact IDs", len(svc.uniqueContactIDs))

	// Free up memory before validation
	svc.uniqueContactIDs = nil
	runtime.GC()

	return nil
}

// validateCSVCounts checks if CSV files have the expected number of records
func (svc *StreamingXMLEscrowService) validateCSVCounts() error {
	baseFilename := svc.GetDepositFileNameWoExtension()

	// Validate main entity CSV files against header expectations
	validationResults := []struct {
		filename     string
		expectedFunc func() int
		description  string
	}{
		{baseFilename + "-domains.csv", svc.Header.DomainCount, "domains"},
		{baseFilename + "-contacts.csv", svc.Header.ContactCount, "contacts"},
		{baseFilename + "-hosts.csv", svc.Header.HostCount, "hosts"},
		{baseFilename + "-registrars.csv", svc.Header.RegistrarCount, "registrars"},
		{baseFilename + "-nndns.csv", svc.Header.NNDNCount, "NNDNs"},
	}

	for _, validation := range validationResults {
		if lines, err := svc.countCSVLines(validation.filename); err != nil {
			log.Printf("⚠️  Could not validate %s: %v", validation.description, err)
		} else {
			expected := validation.expectedFunc()
			actual := lines - 1 // Subtract 1 for header
			if actual != expected {
				log.Printf("⚠️  %s CSV validation: file has %d records (expected %d)", validation.description, actual, expected)
			} else {
				log.Printf("✅ %s CSV validation: %d records match header expectation", validation.description, actual)
			}
		}
	}

	// Validate counter-based CSV files against internal counters
	counterValidations := []struct {
		filename    string
		counter     int
		description string
	}{
		{baseFilename + "-domainStatuses.csv", svc.csvWriters.domainStatusCounter, "domain statuses"},
		{baseFilename + "-domainNameservers.csv", svc.csvWriters.domainNameserverCounter, "domain nameservers"},
		{baseFilename + "-domainTransfers.csv", svc.csvWriters.domainTransferCounter, "domain transfers"},
		{baseFilename + "-DomainDnssec.csv", svc.csvWriters.domainDnssecCounter, "domain DNSSEC records"},
		{baseFilename + "-domainRgpStatus.csv", svc.csvWriters.domainRgpStatusCounter, "domain RGP statuses"},
		{baseFilename + "-contactStatuses.csv", svc.csvWriters.contactStatusCounter, "contact statuses"},
		{baseFilename + "-contactPostalInfo.csv", svc.csvWriters.contactPostalInfoCounter, "contact postal info"},
		{baseFilename + "-hostStatuses.csv", svc.csvWriters.hostStatusCounter, "host statuses"},
		{baseFilename + "-hostAddresses.csv", svc.csvWriters.hostAddressCounter, "host addresses"},
	}

	for _, validation := range counterValidations {
		if lines, err := svc.countCSVLines(validation.filename); err != nil {
			log.Printf("⚠️  Could not validate %s: %v", validation.description, err)
		} else {
			actual := lines - 1 // Subtract 1 for header
			if actual != validation.counter {
				log.Printf("⚠️  %s CSV validation: file has %d records (counter shows %d)", validation.description, actual, validation.counter)
			} else {
				log.Printf("✅ %s CSV validation: %d records match counter", validation.description, actual)
			}
		}
	}

	// Check unique contact IDs file - Skipped as map is cleared for memory optimization
	/*
		uniqueContactFile := baseFilename + "-uniqueDomainContactIDs.csv"
		if lines, err := svc.countCSVLines(uniqueContactFile); err != nil {
			log.Printf("⚠️  Could not validate unique contact IDs: %v", err)
		} else {
			// Log count for info only
			actual := lines - 1
			log.Printf("ℹ️  Unique contact IDs CSV contains %d records", actual)
		}
	*/

	return nil
}

func (svc *StreamingXMLEscrowService) countCSVLines(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Use buffered reader to count lines efficiently
	reader := bufio.NewReader(file)
	count := 0
	buf := make([]byte, 32*1024)
	lineSep := []byte{'\n'}

	for {
		c, err := reader.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

// Cleaned up - removed unused handler structs since we're using direct processing

// Simple processing methods that mirror the original analysis pattern
func (svc *StreamingXMLEscrowService) processDepositTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Extract deposit attributes
	for _, attr := range startElement.Attr {
		switch attr.Name.Local {
		case "type":
			svc.Deposit.Type = attr.Value
		case "id":
			svc.Deposit.ID = attr.Value
		case "prevId":
			svc.Deposit.PrevID = attr.Value
		case "resend":
			// Convert string to int - simplified for now
			if attr.Value != "" && attr.Value != "0" {
				svc.Deposit.Resend = 1 // simplified parsing
			}
		}
	}

	// Don't manually parse deposit content - let the main loop handle all child elements
	// The watermark will be processed by the main loop when it encounters it

	log.Printf("✅ Processed deposit tag - Type: %s, ID: %s", svc.Deposit.Type, svc.Deposit.ID)
	return nil
}

func (svc *StreamingXMLEscrowService) processHeaderTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same logic as original AnalyzeHeaderTag
	if err := decoder.DecodeElement(&svc.Header, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}
	return nil
}

func (svc *StreamingXMLEscrowService) processDomainTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same core logic as ExtractDomains but simplified for single element
	var domain entities.RDEDomain
	if err := decoder.DecodeElement(&domain, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}

	// Track unique contact IDs (from original ExtractDomains logic)
	if svc.uniqueContactIDs == nil {
		svc.uniqueContactIDs = make(map[string]bool)
	}
	svc.uniqueContactIDs[domain.Registrant] = true
	for _, contact := range domain.Contact {
		svc.uniqueContactIDs[contact.ID] = true
	}

	// Update registrar mapping (from original ExtractDomains logic)
	objCount := svc.RegistrarMapping[domain.ClID]
	objCount.DomainCount++
	svc.RegistrarMapping[domain.ClID] = objCount

	// Write to CSV files (same as original ExtractDomains)
	if svc.csvWriters != nil {
		// Write domain to main domain CSV
		svc.csvWriters.domainWriter.Write(domain.ToCSV())

		// Write domain statuses
		for _, status := range domain.Status {
			svc.csvWriters.domainStatusWriter.Write([]string{domain.Name.String(), status.S})
			svc.csvWriters.domainStatusCounter++
		}

		// Write nameservers
		for _, nsGroup := range domain.Ns {
			for _, hostObj := range nsGroup.HostObjs {
				svc.csvWriters.domainNameserverWriter.Write([]string{domain.Name.String(), hostObj})
				svc.csvWriters.domainNameserverCounter++
			}
		}

		// Write DNSSEC data if present (simplified check)
		if len(domain.SecDNS.DSData) > 0 {
			// Create a simple DNSSEC record - adjust fields as needed
			dnssecRecord := []string{domain.Name.String(), "dnssec_data_present"}
			svc.csvWriters.domainDnssecWriter.Write(dnssecRecord)
			svc.csvWriters.domainDnssecCounter++
		}

		// Write transfer data if present
		if domain.TrnData.TrStatus.State != "" {
			transferRecord := []string{
				domain.Name.String(),
				domain.TrnData.TrStatus.State,
				domain.TrnData.ReRr.RegID,
				domain.TrnData.ReDate,
				domain.TrnData.AcRr.RegID,
				domain.TrnData.AcDate,
				domain.TrnData.ExDate,
			}
			svc.csvWriters.domainTransferWriter.Write(transferRecord)
			svc.csvWriters.domainTransferCounter++
		}

		// Write RGP Status if present
		if len(domain.RgpStatus) != 0 {
			for _, rgpStatus := range domain.RgpStatus {
				svc.csvWriters.domainRgpStatusWriter.Write(rgpStatus.ToCSV(domain.Name.String()))
				svc.csvWriters.domainRgpStatusCounter++
			}
		}
	}

	return nil
}

func (svc *StreamingXMLEscrowService) processContactTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same core logic as ExtractContacts but simplified for single element
	var contact entities.RDEContact
	if err := decoder.DecodeElement(&contact, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}

	// Update registrar mapping (from original ExtractContacts logic)
	objCount := svc.RegistrarMapping[contact.ClID]
	objCount.ContactCount++
	svc.RegistrarMapping[contact.ClID] = objCount

	// Write to CSV files
	if svc.csvWriters != nil {
		// Write contact to main contact CSV
		svc.csvWriters.contactWriter.Write(contact.ToCSV())

		// Write contact statuses
		for _, status := range contact.Status {
			svc.csvWriters.contactStatusWriter.Write([]string{contact.ID, status.S})
			svc.csvWriters.contactStatusCounter++
		}

		// Write postal info
		for _, postalInfo := range contact.PostalInfo {
			postalRecord := []string{contact.ID}
			postalRecord = append(postalRecord, postalInfo.ToCSV()...)
			svc.csvWriters.contactPostalInfoWriter.Write(postalRecord)
			svc.csvWriters.contactPostalInfoCounter++
		}
	}

	return nil
}

func (svc *StreamingXMLEscrowService) processHostTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same core logic as ExtractHosts but simplified for single element
	var host entities.RDEHost
	if err := decoder.DecodeElement(&host, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}

	// Update registrar mapping (from original ExtractHosts logic)
	objCount := svc.RegistrarMapping[host.ClID]
	objCount.HostCount++
	svc.RegistrarMapping[host.ClID] = objCount

	// Write to CSV files
	if svc.csvWriters != nil {
		// Write host to main host CSV
		svc.csvWriters.hostWriter.Write(host.ToCSV())

		// Write host statuses
		for _, status := range host.Status {
			svc.csvWriters.hostStatusWriter.Write([]string{host.Name, status.S})
			svc.csvWriters.hostStatusCounter++
		}

		// Write host addresses
		for _, addr := range host.Addr {
			svc.csvWriters.hostAddressWriter.Write([]string{host.Name, addr.IP, addr.ID})
			svc.csvWriters.hostAddressCounter++
		}
	}

	return nil
}

func (svc *StreamingXMLEscrowService) processRegistrarTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same logic as original AnalyzeRegistrarTags
	var registrar entities.RDERegistrar
	if err := decoder.DecodeElement(&registrar, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}

	// Write to CSV if streaming
	if svc.csvWriters != nil {
		// Write registrar data
		svc.csvWriters.registrarWriter.Write(registrar.ToCSV())

		// Write postal info data
		for _, postalInfo := range registrar.PostalInfo {
			svc.csvWriters.registrarPostalInfoWriter.Write(postalInfo.ToCSV(registrar.ID))
		}
	}

	svc.Registrars = append(svc.Registrars, registrar)
	return nil
}

func (svc *StreamingXMLEscrowService) processIDNTableRefTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same logic as original AnalyzeIDNTableRefTags
	var idnTableRef entities.RDEIdnTableReference
	if err := decoder.DecodeElement(&idnTableRef, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}
	// Just count for now - can store if needed later
	return nil
}

func (svc *StreamingXMLEscrowService) processNNDNTag(decoder *xml.Decoder, startElement xml.StartElement) error {
	// Same logic as original ExtractNNDNS but simplified for single element
	var nndn entities.RDENNDN
	if err := decoder.DecodeElement(&nndn, &startElement); err != nil {
		return errors.Join(ErrDecodingXML, err)
	}

	// Write to CSV file
	if svc.csvWriters != nil {
		svc.csvWriters.nndnWriter.Write(nndn.ToCSV())
	}

	return nil
}
