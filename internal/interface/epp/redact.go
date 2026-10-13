package epp

import "github.com/beevik/etree"

// redacted replaces secrets in audit records.
const redacted = "[redacted]"

// redactedFrame serialises doc for the audit log with every secret replaced:
// the text of any <pw> or <newPW> element, and everything inside any
// <authInfo>, whatever their namespace. doc itself is not changed.
func redactedFrame(doc *etree.Document) string {
	c := doc.Copy()
	if root := c.Root(); root != nil {
		redactSecrets(root)
	}
	b, err := c.WriteToBytes()
	if err != nil {
		return ""
	}
	return string(b)
}

func redactSecrets(el *etree.Element) {
	switch el.Tag {
	case "pw", "newPW", "authInfo":
		redactText(el)
		return
	}
	for _, kid := range el.ChildElements() {
		redactSecrets(kid)
	}
}

// redactText replaces all character data under el, keeping its elements so
// the record still shows the shape of what was sent.
func redactText(el *etree.Element) {
	for _, tok := range el.Child {
		switch t := tok.(type) {
		case *etree.CharData:
			if !t.IsWhitespace() {
				t.SetData(redacted)
			}
		case *etree.Element:
			redactText(t)
		}
	}
}
