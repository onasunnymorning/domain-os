package entities

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// <simpleType name="roidType">
//
//		<restriction base="token">
//	  	<pattern value="(\w|_){1,80}-\w{1,8}"/>
//		</restriction>
//
// </simpleType>

// We define a roid as such: {SnowFlakeID int64}_{objectIdentifier string}-{systemIdentifier string}
const (
	ROID_REGEX = `^(\w|_){1,80}-\w{1,8}$`

	SYSTEM_ROID_ID = "APEX" // TODO: make this an ENVAR

	CONTACT_ROID_ID = "CONT"
	HOST_ROID_ID    = "HOST"
	DOMAIN_ROID_ID  = "DOM"

	RoidTypeContact = "contact"
	RoidTypeHost    = "host"
	RoidTypeDomain  = "domain"
)

var (
	ErrInvalidRoid             = errors.New("invalid roid")
	ErrInvalidObjectIdentifier = errors.New("invalid object identifier: accepts ('contact', 'host', 'domain') only")
)

// RoidTypeInt is a type for the Roid
type RoidType string

// NewRoidType creates a new instance of RoidType based on a snowflake ID + object identifier + system identifier
func NewRoidType(uniqueID int64, objectIdentifier string) (RoidType, error) {
	switch objectIdentifier {
	case RoidTypeContact:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, CONTACT_ROID_ID, SYSTEM_ROID_ID)), nil
	case RoidTypeHost:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, HOST_ROID_ID, SYSTEM_ROID_ID)), nil
	case RoidTypeDomain:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, DOMAIN_ROID_ID, SYSTEM_ROID_ID)), nil
	default:
		return RoidType(""), ErrInvalidObjectIdentifier
	}
}

// Validate checks if the RoidType is valid
func (r RoidType) Validate() error {
	regex := regexp.MustCompile(ROID_REGEX)
	if !regex.MatchString(string(r)) {
		return ErrInvalidRoid
	}
	return nil
}

// String implements the Stringer interface
func (r RoidType) String() string {
	return string(r)
}

// Int64 returns the Unique ID part of the RoidType
func (r RoidType) Int64() (int64, error) {
	strInt64 := strings.Split(string(r), "_")[0]
	return strconv.ParseInt(strInt64, 10, 64)
}

// ObjectIdentifier returns the object identifier part of the RoidType
func (r RoidType) ObjectIdentifier() string {
	return strings.Split(strings.Split(string(r), "_")[1], "-")[0]
}

// SystemIdentifier returns the system identifier part of the RoidType
func (r RoidType) SystemIdentifier() string {
	return strings.Split(string(r), "-")[1]
}
