// Convert a pacs.008 XML document from stdin into a canonical transfer request.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/apache261/Byakugan-core/domain"
	"github.com/apache261/Byakugan-core/iso20022"
)

func main() {
	idempotencyKey := flag.String("idempotency-key", "", "caller-supplied replay key")
	flag.Parse()
	log.SetFlags(0)

	request, err := iso20022.ParsePacs008(os.Stdin, *idempotencyKey)
	if err != nil {
		log.Fatal(err)
	}
	if validationErrors := domain.ValidateTransfer(request); len(validationErrors) > 0 {
		log.Fatal(domain.ValidationReason(validationErrors))
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(request); err != nil {
		log.Fatal(err)
	}
}
