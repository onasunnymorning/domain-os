// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package validation

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/lestrrat-go/libxml2"
	"github.com/lestrrat-go/libxml2/xsd"
	"github.com/pkg/errors"
)

// XMLValidator is an XML validator which can validate using XSD files
// and/or custom validation rules.
type XMLValidator struct {
	XSDIndexFile string

	parseSchemaOnce sync.Once
	xsdSchema       *xsd.Schema
}

// ValidateXML validate the given schema to the configured XSD files.
func (c *XMLValidator) ValidateXML(doc []byte) error {
	c.parseSchemaOnce.Do(func() {
		s, err := xsd.ParseFromFile(c.XSDIndexFile)
		if err != nil {
			wd, _ := os.Getwd()
			panic(fmt.Sprintf("wd: %s, err: %v", wd, err))
		}

		c.xsdSchema = s
	})

	libxml2Doc, err := libxml2.Parse(doc)
	if err != nil {
		return err
	}

	defer libxml2Doc.Free()

	if err = c.xsdSchema.Validate(libxml2Doc); err != nil {
		var vErr xsd.SchemaValidationError

		errs := []string{}

		if !errors.As(err, &vErr) {
			return err
		}

		for _, e := range vErr.Errors() {
			errs = append(errs, e.Error())
		}

		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}
