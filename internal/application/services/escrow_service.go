package services

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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

// getXMLDecoder opens the XML file and returns an XML decoder
func (svc *XMLEscrowAnalysisService) getXMLDecoder() (*xml.Decoder, error) {
	f, err := os.Open(svc.Deposit.FileName)
	if err != nil {
		return nil, err
	}
	return xml.NewDecoder(f), nil
}
