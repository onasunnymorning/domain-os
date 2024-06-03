// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package prompt

import (
	"bufio"

	"github.com/dotse/epp-client/pkg"
	"github.com/dotse/epp-client/pkg/validation"
)

// Prompt handles the prompt interface to the epp client.
// Present options for commands and the corresponding data to be sent to the server.
type Prompt struct {
	Client            *pkg.Client
	Templates         pkg.Templates
	MultilineScanner  *bufio.Scanner
	ValidateResponses bool
	XMLValidator      *validation.XMLValidator
	Cli               Output
}

// Output a collection of functions needed for output.
type Output interface {
	Println(...interface{})
}
