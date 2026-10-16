package entities

import "errors"

// Refusals of EPP commands on host and contact objects alike. The
// application layer wraps them with the object and the status.
var (
	// ErrEPPStatusNotClient: adding or removing a status a client does not
	// control.
	ErrEPPStatusNotClient = errors.New("a client cannot set or remove that status")
	// ErrEPPStatusAlreadySet: adding a status the object already has.
	ErrEPPStatusAlreadySet = errors.New("the object already has the status")
	// ErrEPPStatusNotSet: removing a status the object does not have.
	ErrEPPStatusNotSet = errors.New("the object does not have the status")
	// ErrEPPObjectPending: a transform of an object with a pending action.
	ErrEPPObjectPending = errors.New("the object has a pending action")
	// ErrEPPDeleteProhibited: a delete of an object with a delete
	// prohibition.
	ErrEPPDeleteProhibited = errors.New("the object's status prohibits deleting it")
)
