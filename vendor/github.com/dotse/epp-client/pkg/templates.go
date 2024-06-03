// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package pkg

import (
	"embed"
	"io"
	"sync"
	"text/template"
)

//go:embed templates/*
var xmlTemplates embed.FS

// Templates hold the templates and template functions.
type Templates struct {
	parseTemplatesOnce sync.Once
	parsedTemplates    *template.Template
}

// Execute the given template using the given data.
func (t *Templates) Execute(w io.Writer, name string, data any) error {
	t.parseTemplatesOnce.Do(func() {
		parsedTemplates := &template.Template{}
		parsedTemplates = parsedTemplates.Funcs(
			template.FuncMap{
				"BoolToInt": func(b bool) int {
					if b {
						return 1
					}

					return 0
				},
			},
		)

		_, err := parsedTemplates.ParseFS(xmlTemplates, "templates/*")
		if err != nil {
			panic(err)
		}

		t.parsedTemplates = parsedTemplates
	})

	return t.parsedTemplates.ExecuteTemplate(w, name, data)
}
