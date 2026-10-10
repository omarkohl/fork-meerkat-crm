package carddav

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emersion/go-vcard"
)

// FuzzVCardToContact feeds arbitrary bytes through the vCard decoder and the mapper,
// as the CardDAV server, the CardDAV client and the VCF import do with untrusted
// input. Whatever decodes must map without panicking, and the mapped contact must
// serialise back into a card that decodes again.
//
// Run: go test ./carddav -run '^$' -fuzz FuzzVCardToContact -fuzztime 30s
func FuzzVCardToContact(f *testing.F) {
	seeds := []string{
		"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;Augusta;Dr.;PhD\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Solo\r\nEMAIL;TYPE=work:a@example.com\r\nTEL;TYPE=cell:+15551234567\r\nBDAY:--0229\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:X\r\nitem1.ADR;TYPE=home:;;1 Main St;Town;;12345;Land\r\nitem1.X-ABLabel:Cabin\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:P\r\nPHOTO;ENCODING=b;TYPE=JPEG:/9j/4AAQSkZJRgABAQEASABIAAD\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:P\r\nPHOTO;VALUE=uri:https://example.com/p.jpg\r\nGENDER:F\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Multi\r\nORG:Acme;Dept\r\nTITLE:CEO\r\nURL:https://example.com\r\nNOTE:line\\nbreak\\, comma\r\nCATEGORIES:a,b\r\nX-CUSTOM:1\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\nVERSION:3.0\nFN:LF only\nEND:VCARD\n",
		"BEGIN:VCARD\r\nEND:VCARD\r\n",
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := vcard.NewDecoder(bytes.NewReader(data))
		for i := 0; i < 8; i++ {
			card, err := dec.Decode()
			if err != nil {
				return
			}
			contact, _, _, _ := VCardToContact(card, nil)
			if contact == nil {
				t.Fatal("VCardToContact returned a nil contact")
			}
			ParseRevision(card)

			// The contact must survive being served again.
			out := ContactToVCard(contact, "")
			var buf bytes.Buffer
			if err := vcard.NewEncoder(&buf).Encode(out); err != nil {
				return // the encoder refuses some cards; that is an error, not a panic
			}
			if !strings.Contains(buf.String(), "BEGIN:VCARD") {
				t.Fatalf("encoded card has no BEGIN: %q", buf.String())
			}
			if _, err := vcard.NewDecoder(&buf).Decode(); err != nil {
				t.Fatalf("encoded card does not decode again: %v\n%q", err, buf.String())
			}
		}
	})
}
