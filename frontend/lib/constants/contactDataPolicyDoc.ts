export const CONTACT_DATA_POLICY_DOC_MARKDOWN = `# Contact Data Policy Reference Guide

The **Contact Data Policy** determines the requirement levels for contact information (Registrant, Tech, Admin, and Billing) during domain registration and updates. Because registry requirements vary wildly across different jurisdictions and business models, Domain OS allows configuring these policies per-phase for each TLD.

---

## Contact Roles

Every domain registration can associate contact details for up to four distinct roles:

1. **Registrant**: The primary owner or licensee of the domain name.
2. **Tech (Technical Contact)**: The entity responsible for technical management of the domain (e.g., DNS, name servers).
3. **Admin (Administrative Contact)**: The business contact authorized to perform operations on behalf of the registrant.
4. **Billing (Billing Contact)**: The contact responsible for invoice generation, payment transactions, and renewals.

---

## Policy Options & Enforcement

For each contact role, you can set one of three enforcement behaviors:

### 1. Mandatory
* **Behavior**: The contact **must** be provided when registering or updating the domain.
* **Enforcement**: If a registrar submits a creation or update request without this contact role, the registry rejects the transaction immediately and throws a \`Contact Data Policy Violation\` error (e.g., \`ErrTechIDRequiredButNotSet\`).

### 2. Optional
* **Behavior**: The contact is permitted but not required.
* **Enforcement**: Registrars can choose whether to supply a contact ID. The system stores it if present, and ignores the field if omitted without raising errors.

### 3. Prohibited
* **Behavior**: The contact is **explicitly forbidden** to be stored on the registry database.
* **Enforcement**: If a registrar submits a command containing a prohibited contact ID, the registry automatically strips (nullifies) the field before writing it to the database, ensuring compliance with strict data protection principles rather than failing the EPP request.

---

## Default Configuration (ICANN 2025 RDP Compliant)

By default, any newly created phase policy uses a standard layout aligned with the **2025 ICANN Registration Data Policy (RDP) for gTLDs**:

| Contact Role | Default Status | Rationale |
| :--- | :--- | :--- |
| **Registrant** | \`Mandatory\` | Basic requirement to establish domain ownership. |
| **Tech** | \`Mandatory\` | Necessary for resolution of network/routing issues. |
| **Admin** | \`Optional\` | Frequently omitted or redacted in modern privacy-focused registries. |
| **Billing** | \`Optional\` | Handled primarily at the registrar tier; optional at the registry. |

---

## UI Management

Registry operators can modify these policy rules on a per-phase basis:

1. Go to the **TLD Dashboard** and click on your target TLD.
2. Open the **Phases** tab and select the phase details.
3. Scroll to the **Contact Data Policy** section where you can view badges representing the current configuration.
4. Click **Edit** in the section header to display dropdown selectors for each role.
5. Save changes to update.

> [vanish]
> Policies are locked for **past phases** to maintain historical consistency and legal traceability. You can only edit policies for **current** and **future** phases.

---

## Practical Examples & Use Cases

### Example 1: Privacy-First TLD (GDPR-Aligned, e.g., European ccTLDs)
For registry operators looking to minimize data storage and adhere to strict privacy guidelines (e.g., GDPR), technical and administrative roles are prohibited at the registry tier.

* **Configuration**:
  * **Registrant**: \`Mandatory\` (for legal accountability)
  * **Tech**: \`Prohibited\` (avoid storing technician personal data)
  * **Admin**: \`Prohibited\` (no business contact stored)
  * **Billing**: \`Prohibited\` (billing handled at registrar level)

* **Behavior**:
  * If a registrar sends an EPP request containing an \`<admin>\` or \`<tech>\` contact element, the registry accepts the request but **silently unsets** the fields.

---

### Example 2: Corporate Brand TLD (High Security / Internal Only, e.g., \`.brand\`)
A brand registry restricted to internal business units might require full accountability and contact info for all roles to prevent unauthorized changes and streamline internal chargebacks.

* **Configuration**:
  * **Registrant**: \`Mandatory\`
  * **Tech**: \`Mandatory\`
  * **Admin**: \`Mandatory\`
  * **Billing**: \`Mandatory\`

* **Behavior**:
  * Any EPP create/update commands that omit *any* of the four contact IDs will be rejected with an EPP error code \`2003\` (Required parameter missing).

---

### Example 3: EPP XML Payload Scenarios

#### Scenario A: Create Domain with Missing Mandatory Field
Assume **Billing** is set to \`Mandatory\`. A registrar sends a create request without a billing contact:

\`\`\`xml
<contact:type="registrant">registrant-id-123</contact:type>
<contact:type="admin">admin-id-456</contact:type>
<contact:type="tech">tech-id-789</contact:type>
<!-- Note: No billing contact is provided -->
\`\`\`

* **Result**: Command fails with:
  \`\`\`json
  {
    "error": "contact data policy violation: billing contact ID is required but not set"
  }
  \`\`\`

#### Scenario B: Create Domain with Prohibited Field
Assume **Admin** is set to \`Prohibited\`. A registrar sends a create request containing an admin contact:

\`\`\`xml
<contact:type="registrant">registrant-id-123</contact:type>
<contact:type="admin">admin-id-456</contact:type>
<contact:type="tech">tech-id-789</contact:type>
\`\`\`

* **Result**: Domain is successfully created. However, querying the domain via EPP info or inspecting the database reveals:
  * \`registrantID\`: \`registrant-id-123\`
  * \`adminID\`: \`""\` (Strips the prohibited field)
  * \`techID\`: \`tech-id-789\`
`;
