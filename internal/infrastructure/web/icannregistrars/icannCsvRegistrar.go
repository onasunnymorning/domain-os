package icannregistrars

import (
	"fmt"
	"strings"

	"github.com/biter777/countries"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// CSVRegistrar represents a registrar in the CSV file from ICANN
// Header: "Registrar Name","IANA Number","Country/Territory","Public Contact","Link"
type CSVRegistrar struct {
	Name          string
	IANAID        int
	Country       string
	PublicContact string
	Link          string
}

// ContactName returns the name of the contact person (the first part of the contact string, before the `+` sign)
func (r CSVRegistrar) ContactName() string {
	if !strings.Contains(r.PublicContact, "+") {
		namestring := strings.Split(r.PublicContact, "null")[0]
		return strings.TrimSpace(namestring)
	}
	namestring := strings.Split(r.PublicContact, "+")[0]
	return strings.TrimSpace(namestring)
}

// CreateSlug creates a slug from the registrar name that is a valid ClIDType
func (r CSVRegistrar) CreateSlug() (string, error) {
	// split the string by comma ',' and return the frist part
	slug := strings.Split(r.Name, ",")[0]
	// lowercase the string
	slug = strings.ToLower(slug)
	// Remove all Non-ASCII characters
	slug = entities.RemoveNonASCII(slug)
	// replace all spaces ' ' with dashes '-'
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove all Non-AlphaNumeric characters
	slug = entities.RemoveNonAlphaNumeric(slug)
	// remove all dots '.'
	slug = strings.ReplaceAll(slug, ".", "")
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// prepend the IANAID to the slug
	slug = fmt.Sprintf("%d-%s", r.IANAID, slug)
	// if the string is longer than 16 characters, truncate it
	if len(slug) > 16 {
		slug = slug[:16]
	}
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// validate as a ClIDType
	clidSlug, err := entities.NewClIDType(slug)
	return clidSlug.String(), err
}

// ContactPhone returns the phone number if the PublicContact contains a '+'.
// It tries to ignore the email chunk (anything containing '@').
func (r CSVRegistrar) ContactPhone() string {
	// Look for a plus sign
	plusIdx := strings.Index(r.PublicContact, "+")
	if plusIdx == -1 {
		// No phone number found
		return ""
	}

	// Extract the substring after the plus sign
	afterPlus := r.PublicContact[plusIdx+1:]

	// If there's an email in afterPlus, we don't want to parse that as phone
	// Find any '@'
	atIdx := strings.Index(afterPlus, "@")
	// We'll define 'phoneCandidate' as everything from the plus sign up to the email (if present)
	var phoneCandidate string
	if atIdx == -1 {
		// No '@' found, so presumably there's no email chunk. The rest is phone
		phoneCandidate = afterPlus
	} else {
		// There's an email somewhere. We only want the phone chunk before the email
		// For example, "Bob Smith +123 4567890 someone@example.com"
		// afterPlus = "123 4567890 someone@example.com"
		// So we can find the position in 'afterPlus' that starts the email token
		// which presumably starts at the last space before @...

		// This is a naive but typical approach:
		//   tokens = ["123", "4567890", "someone@example.com"]
		//   so we take tokens up to the last token that contains '@'
		tokens := strings.Fields(afterPlus) // space split
		var phoneTokens []string
		for _, t := range tokens {
			if strings.Contains(t, "@") {
				break
			}
			phoneTokens = append(phoneTokens, t)
		}
		phoneCandidate = strings.Join(phoneTokens, " ")
	}

	// Now we have something like "123 4567890"
	// Let's clean out non-digits but preserve the first space to convert to a '.'
	cleaned := cleanPhoneNumber([]byte(phoneCandidate))
	// If there's a space, replace the first space with '.' to match your test's "123.4567890"
	cleaned = strings.Replace(cleaned, " ", ".", 1)
	// Remove subsequent spaces
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Validate
	validated, err := entities.NewE164Type("+" + cleaned)
	if err != nil {
		// Possibly log it, but as your code does: just return empty string
		return ""
	}
	return validated.String()
}

// cleanPhoneNumber removes all characters from the phone number string that are not numbers
func cleanPhoneNumber(s []byte) string {
	j := 0
	for _, b := range s {
		if ('0' <= b && b <= '9') || b == ' ' {
			s[j] = b
			j++
		}
	}
	return string(s[:j])
}

// ContactEmail returns the email of the contact person (the last part of the contact string)
func (r CSVRegistrar) ContactEmail() string {
	tokens := strings.Fields(r.PublicContact) // splits on whitespace
	for i := len(tokens) - 1; i >= 0; i-- {
		if strings.Contains(tokens[i], "@") {
			return tokens[i]
		}
	}
	return "" // no email found
}

// CountryCode returns the country code of the registrar
func (r CSVRegistrar) Address() (*entities.Address, error) {
	country := countries.ByName(r.Country)
	// There are some exceptions in the file
	if strings.Contains(r.Country, "United Kingdom") {
		country = countries.ByName("United Kingdom")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Hong Kong") {
		country = countries.ByName("Hong Kong")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Marshall Islands") {
		country = countries.ByName("Marshall Islands")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Panama") {
		country = countries.ByName("Panama")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	if strings.Contains(r.Country, "Taipei") {
		country = countries.ByName("Taiwan")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}
	// 2024-04-13 - The following entry contains an empty country field so adding a manual check for the IANAID 3874:
	// "Butterfly Asset Management Pte. Ltd",3874,,"Jianwen Chen +65 83516253 birichcom@163.com","http://birich.com"
	if r.IANAID == 3874 {
		country = countries.ByName("Singapore")
		if !country.IsValid() {
			return nil, fmt.Errorf("country not found: %s", r.Country)
		}
	}

	if !country.IsValid() {
		return nil, fmt.Errorf("country not found: %s", r.Country)
	}

	return &entities.Address{
		City:        entities.PostalLineType(country.Capital().Info().Name),
		CountryCode: entities.CCType(country.Alpha2()),
	}, nil
}
