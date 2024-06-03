// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package prompt

import (
	"context"
	"io"

	"github.com/dotse/epp-client/pkg"
)

// Run start the epp client interface.
// Provides options for which command and data to send.
func (p *Prompt) Run(ctx context.Context, validateResponses bool) {
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		// get the request information to send to the server
		name, template, data := p.setupRequest(pkg.CommandConfig)

		var (
			response string
			err      error
		)

		switch name {
		case exit:
			// exit the program
			return

		case customXML:
			// send whatever the user put in
			response, err = p.Client.SendCommand(data.([]byte))
			if err != nil {
				panic(err)
			}

		default:
			response, err = p.Client.SendCommandUsingTemplate(template, data)
		}

		if err != nil {
			panic(err)
		}

		p.Cli.Println(response)

		if validateResponses {
			if err := p.XMLValidator.ValidateXML([]byte(response)); err != nil {
				p.Cli.Println(err.Error())
			} else {
				p.Cli.Println("ok")
			}
		}
	}
}

func (p *Prompt) printRequest(w io.Writer, name string, data any) {
	if err := p.Templates.Execute(w, name, data); err != nil {
		panic(err)
	}
}
