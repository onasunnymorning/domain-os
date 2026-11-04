package schema

import (
	"testing"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Text reads what validation validated: every character-data child, with
// comments and processing instructions skipped. etree's Element.Text stops
// at the first of those, which is how a valid value could read as empty.
func TestText(t *testing.T) {
	for raw, want := range map[string]string{
		`<p>landrush</p>`:                    "landrush",
		`<p><?x?>landrush</p>`:               "landrush",
		`<p>land<!-- c -->rush</p>`:          "landrush",
		`<p> land<?x y?>rush <!-- c --></p>`: " landrush ",
		`<p><!-- only a comment --></p>`:     "",
		`<p/>`:                               "",
	} {
		t.Run(raw, func(t *testing.T) {
			doc := etree.NewDocument()
			require.NoError(t, doc.ReadFromString(raw))
			el := doc.Root()
			assert.Equal(t, want, Text(el))
		})
	}
	assert.Equal(t, "", Text(nil))
}
