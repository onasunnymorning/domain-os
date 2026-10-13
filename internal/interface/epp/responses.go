package epp

import (
	"fmt"
	"time"
)

// The response builders below are string templates carried over unchanged
// from the stub. Transaction IDs are placeholders; a unique svTRID and an
// echoed clTRID are Phase 1 of docs/epp/ARCHITECTURE.md.

func greetingXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <greeting>
    <svID>EPP Server</svID>
    <svDate>` + time.Now().UTC().Format(time.RFC3339) + `</svDate>
    <svcMenu>
      <version>1.0</version>
      <lang>en</lang>
      <objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:host-1.0</objURI>
    </svcMenu>
  </greeting>
</epp>`
}

func authErrorXML(reason string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="2200">
      <msg>Authentication error - ` + reason + `</msg>
    </result>
    <trID>
      <svTRID>` + fmt.Sprintf("srv-%d", time.Now().UnixNano()) + `</svTRID>
    </trID>
  </response>
</epp>`
}

func loginResponseXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <trID>
      <clTRID>ABC-12345</clTRID>
      <svTRID>APEX-LOGIN-123</svTRID>
    </trID>
  </response>
</epp>`
}

func logoutResponseXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1500">
      <msg>Command completed successfully; ending session</msg>
    </result>
    <trID>
      <clTRID>ABC-12345</clTRID>
      <svTRID>APEX-LOGOUT-123</svTRID>
    </trID>
  </response>
</epp>`
}

func placeholderDomainCheckXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="1000"><msg>Welcome Stranger</msg></result><resData><domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:cd><domain:name avail="1">geoff.smoketestcnic</domain:name></domain:cd></domain:chkData></resData> <trID><clTRID>ABC-12345</clTRID><svTRID>APEX-123</svTRID></trID></response></epp>`
}
