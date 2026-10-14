package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Event authInfo redaction
//
// Drives every domain and contact producer that carries an authInfo — contact
// create/update/delete, domain register/update/delete — through the API, then
// scans the domain_events rows those calls wrote for the secrets. Each secret
// is unique to this file so a hit cannot come from another spec.
var _ = Describe("EventAuthInfoRedaction", Ordered, func() {
	const (
		ryID           = "erTestRyOp"
		tldName        = "ertest"
		registrarClID  = "erTestRar"
		registrarGurID = 10021
		contactID      = "erTestCont"
		domainName     = "redact.ertest"

		contactSecret       = "Er!C0ntactSecr3t"
		domainSecret        = "Er!D0mainSecr3t"
		updatedDomainSecret = "Er!D0mainR0tat3d"
	)
	secrets := []string{contactSecret, domainSecret, updatedDomainSecret}

	BeforeAll(func() {
		resp := api.POST("/registry-operators", &commands.CreateRegistryOperatorCommand{
			RyID: ryID, Name: "Event Redaction Test Registry Operator", Email: "admin@ertest.com",
		})
		Expect(resp.Code).To(Equal(http.StatusCreated))

		resp = api.POST("/tlds", &request.CreateTLDRequest{Name: tldName, RyID: ryID})
		Expect(resp.Code).To(Equal(http.StatusCreated))

		resp = api.POST(fmt.Sprintf("/tlds/%s/phases", tldName), &commands.CreatePhaseCommand{
			Name: "GA1", Type: "GA", Starts: time.Now().UTC().Add(-48 * time.Hour),
		})
		Expect(resp.Code).To(Equal(http.StatusCreated))

		resp = api.POST("/registrars", testRegistrar(registrarClID, "Event Redaction Test Registrar", registrarGurID))
		Expect(resp.Code).To(Equal(http.StatusCreated))
		resp = api.PUT(fmt.Sprintf("/registrars/%s/status/ok", registrarClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
		resp = api.PUT(fmt.Sprintf("/registrars/%s/iana_status/Accredited", registrarClID), nil)
		Expect(resp.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusNoContent)))
		resp = api.POSTNoBody(fmt.Sprintf("/accreditations/%s/%s", tldName, registrarClID))
		Expect(resp.Code).To(Equal(http.StatusCreated))
	})

	It("writes contact events without the contact authInfo", func() {
		addr, err := entities.NewAddress("Test City", "US")
		Expect(err).NotTo(HaveOccurred())
		postal, err := entities.NewContactPostalInfo("int", "ER Test Contact", addr)
		Expect(err).NotTo(HaveOccurred())

		resp := api.POST("/contacts", &commands.CreateContactCommand{
			ID: contactID, RoID: "10021_CONT-APEX", Email: "ertest@example.com",
			AuthInfo: contactSecret, ClID: registrarClID,
			PostalInfo: [2]*entities.ContactPostalInfo{postal},
		})
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())

		var created entities.Contact
		Expect(DecodeJSON(resp, &created)).To(Succeed())
		created.Email = "ertest-updated@example.com"
		resp = api.PUT(fmt.Sprintf("/contacts/%s", contactID), created)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())

		expectRedactedEvents(contactID, "contact.created", "contact.updated")
	})

	It("writes domain events without the domain authInfo", func() {
		resp := api.POST(fmt.Sprintf("/domains/%s/register", domainName), &commands.RegisterDomainCommand{
			Name: domainName, ClID: registrarClID, AuthInfo: domainSecret,
			RegistrantID: contactID, AdminID: contactID, TechID: contactID, BillingID: contactID,
			Years: 1,
		})
		Expect(resp.Code).To(Equal(http.StatusCreated), resp.Body.String())

		resp = api.GET(fmt.Sprintf("/domains/%s", domainName))
		Expect(resp.Code).To(Equal(http.StatusOK))
		var dom entities.Domain
		Expect(DecodeJSON(resp, &dom)).To(Succeed())
		upd := &commands.UpdateDomainCommand{}
		upd.FromEntity(&dom)
		upd.AuthInfo = updatedDomainSecret
		resp = api.PUT(fmt.Sprintf("/domains/%s", domainName), upd)
		Expect(resp.Code).To(Equal(http.StatusOK), resp.Body.String())

		resp = api.DELETEAs(fmt.Sprintf("/domains/%s", domainName), ryID)
		Expect(resp.Code).To(Equal(http.StatusNoContent), resp.Body.String())

		expectRedactedEvents(domainName, "domain.registered", "domain.updated", "domain.admin_deleted")
	})

	It("writes the contact delete event without the contact authInfo", func() {
		resp := api.DELETE(fmt.Sprintf("/contacts/%s", contactID))
		Expect(resp.Code).To(Equal(http.StatusNoContent), resp.Body.String())

		expectRedactedEvents(contactID, "contact.deleted")
	})

	It("leaves no secret in any event row", func() {
		for _, secret := range secrets {
			var n int64
			Expect(api.DB.Model(&postgres.DomainEventRecord{}).
				Where("data::text LIKE ? OR command::text LIKE ? OR before_state::text LIKE ? OR after_state::text LIKE ?",
					"%"+secret+"%", "%"+secret+"%", "%"+secret+"%", "%"+secret+"%").
				Count(&n).Error).To(Succeed())
			Expect(n).To(BeZero(), "an event row contains authInfo %q", secret)
		}
	})

	AfterAll(func() {
		api.DELETE(fmt.Sprintf("/contacts/%s", contactID))
		api.DELETE(fmt.Sprintf("/registrars/%s", registrarClID))
		api.DELETEAs(fmt.Sprintf("/tlds/%s", tldName), ryID)
		api.DELETE(fmt.Sprintf("/registry-operators/%s", ryID))
	})
})

// expectRedactedEvents asserts that subject has an event of each wantType and
// that every authInfo in those events' payloads is the redaction marker.
func expectRedactedEvents(subject string, wantTypes ...string) {
	GinkgoHelper()
	var rows []postgres.DomainEventRecord
	Expect(api.DB.Where("subject = ? AND type IN ?", subject, wantTypes).Find(&rows).Error).To(Succeed())

	seen := map[string]bool{}
	markers := 0
	for _, r := range rows {
		seen[r.Type] = true
		for _, payload := range [][]byte{r.Data, r.Command, r.BeforeState, r.AfterState} {
			if len(payload) == 0 {
				continue
			}
			var tree interface{}
			Expect(json.Unmarshal(payload, &tree)).To(Succeed())
			markers += countAuthInfoMarkers(tree, r.Type)
		}
	}
	for _, t := range wantTypes {
		Expect(seen).To(HaveKey(t), "no %s event for %s", t, subject)
	}
	// Every one of these events carries at least one entity or command with an
	// authInfo, so the marker must appear — proof the field was redacted rather
	// than the payload simply missing.
	Expect(markers).To(BeNumerically(">=", len(wantTypes)))
}

// countAuthInfoMarkers counts AuthInfo fields holding the redaction marker and
// fails on any that hold something else.
func countAuthInfoMarkers(node interface{}, eventType string) int {
	GinkgoHelper()
	n := 0
	switch v := node.(type) {
	case map[string]interface{}:
		for k, child := range v {
			if k == "AuthInfo" {
				Expect(child).To(Equal(postgres.RedactedMarker), "%s event carries an unredacted AuthInfo", eventType)
				n++
				continue
			}
			n += countAuthInfoMarkers(child, eventType)
		}
	case []interface{}:
		for _, child := range v {
			n += countAuthInfoMarkers(child, eventType)
		}
	}
	return n
}
