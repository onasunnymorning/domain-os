package escrow

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// EscrowImportController is a controller for escrow analysis
type EscrowImportController struct {
	svc *services.XMLEscrowService
}

// NewEscrowImportController creates a new instance of EscrowAnalysisController
func NewEscrowImportController(escrowService *services.XMLEscrowService) *EscrowImportController {
	return &EscrowImportController{
		svc: escrowService,
	}
}

// Import calls the escrow analysis service to import the data into the database
func (c *EscrowImportController) Import(analysisFile, depositFile string) error {

	// Load the analysis file

	err := c.svc.LoadDepostiAnalysis(analysisFile, depositFile)
	if err != nil {
		return err
	}

	// Load the unique contact IDs from file

	err = c.svc.LoadUniqueContactIDs()
	if err != nil {
		return err
	}

	// Check the TLD is in a state allowing import
	// TODO: Implement thi https://github.com/onasunnymorning/domain-os/issues/60
	tld, err := c.svc.GetTLDFromAPI(c.svc.Header.TLD)
	if err != nil || tld == nil {
		return err
	}
	log.Println("Found TLD in API")

	// Import the Contacts if the number of contacts > 0
	if c.svc.Header.ContactCount() > 0 {
		contactCmds, err := c.svc.ExtractContacts(true)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
		err = c.svc.CreateContacts(contactCmds)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
	}

	// Import the NNDNs if the number of NNDNs > 0
	if c.svc.Header.NNDNCount() > 0 {
		nndnCmds, err := c.svc.ExtractNNDNS(true)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
		err = c.svc.CreateNNDNs(nndnCmds)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
	} else {
		log.Println("No NNDNs to import")
	}

	// Import the Hosts if the number of Hosts > 0
	if c.svc.Header.HostCount() > 0 {
		hostCmds, err := c.svc.ExtractHosts(true)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
		log.Printf("Hosts count from deposit: %d\n", len(hostCmds))
		log.Println("Duplicating Hosts where needed to ensure correct sponsorship")
		// See if we need to duplicate some hosts so we can respect the strict sponsorship enforcement
		hostCmds, err = c.svc.DuplicateHostCommands(hostCmds)
		log.Printf("Hosts after correction: %d\n", len(hostCmds))
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
		err = c.svc.CreateHosts(hostCmds)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
	} else {
		log.Println("No Hosts to import")
	}

	// Import the Domains if the number of Domains > 0
	if c.svc.Header.DomainCount() > 0 {
		domainCmds, err := c.svc.ExtractDomains(true)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
		err = c.svc.CreateDomains(domainCmds)
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
	} else {
		log.Println("No Domains to import")
	}

	// Link the Hosts to the Domains
	if c.svc.Header.DomainCount() == 0 || c.svc.Header.HostCount() == 0 {
		// TODO: Set all domains to inactive
		log.Println("No Hosts to link to Domains")
	} else {
		err = c.svc.LinkHostsToDomains()
		if err != nil {
			c.svc.SaveImportResult()
			return err
		}
	}

	// QA the import was successful
	// TODO: FIXME: Implement this

	// Log the result in a file
	err = c.svc.SaveImportResult()
	if err != nil {
		return err
	}

	return nil
}
