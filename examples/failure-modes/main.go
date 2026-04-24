// Example: hard and soft ingest failure modes
package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
)

type attemptedDocument struct {
	docID gin.DocID
	body  string
}

var attemptedDocuments = []attemptedDocument{
	{docID: gin.DocID(0), body: `{"email":"alice@example.com","score":9007199254740993}`},
	{docID: gin.DocID(1), body: `{"email":42}`},
	{docID: gin.DocID(2), body: `{"email":"bob@example.com","score":1.5}`},
	{docID: gin.DocID(3), body: `not-json`},
	{docID: gin.DocID(4), body: `{"email":"carol@example.com","score":9007199254740992}`},
}

const (
	hardStoppedAfterOneDocument = "hard: stopped after 1 indexed document"
	softSkippedTwoDocuments     = "soft: skipped 2 documents"
	softIndexedThreeDocuments   = "soft: indexed 3 documents"
	softDomainRowGroupsPrefix   = "soft: email-domain example.com row groups"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := runHard(); err != nil {
		return err
	}
	return runSoft()
}

func runHard() error {
	config, err := gin.NewConfig(
		gin.WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		return errors.Wrap(err, "create hard config")
	}
	builder, err := gin.NewBuilder(config, len(attemptedDocuments))
	if err != nil {
		return errors.Wrap(err, "create hard builder")
	}

	indexed := 0
	for _, doc := range attemptedDocuments {
		// The hard config stops at DocID(1) because {"email":42} triggers
		// transformer rejection after DocID(0) was accepted.
		if err := builder.AddDocument(doc.docID, []byte(doc.body)); err != nil {
			if indexed != 1 {
				return errors.Errorf("hard config stopped after %d indexed documents, want 1", indexed)
			}
			fmt.Printf("%s: %v\n", hardStoppedAfterOneDocument, err)
			return nil
		}
		indexed++
	}

	return errors.New("hard config accepted all attempted documents")
}

func runSoft() error {
	config, err := gin.NewConfig(
		gin.WithParserFailureMode(gin.IngestFailureSoft),
		gin.WithNumericFailureMode(gin.IngestFailureSoft),
		gin.WithEmailDomainTransformer(
			"$.email",
			"domain",
			gin.WithTransformerFailureMode(gin.IngestFailureSoft),
		),
	)
	if err != nil {
		return errors.Wrap(err, "create soft config")
	}
	builder, err := gin.NewBuilder(config, len(attemptedDocuments))
	if err != nil {
		return errors.Wrap(err, "create soft builder")
	}

	for _, doc := range attemptedDocuments {
		if err := builder.AddDocument(doc.docID, []byte(doc.body)); err != nil {
			return errors.Wrapf(err, "add soft document %d", doc.docID)
		}
	}

	// The soft config skips malformed JSON and incompatible numeric promotion,
	// but companion transformer soft failures keep the raw source document.
	// Matching domain rows therefore land in dense row groups [0 2].
	idx := builder.Finalize()
	if builder.NumSoftSkippedDocuments() != 2 {
		return errors.Errorf("soft skipped %d documents, want 2", builder.NumSoftSkippedDocuments())
	}
	result := idx.Evaluate([]gin.Predicate{
		gin.EQ("$.email", gin.As("domain", "example.com")),
	})
	if idx.Header.NumDocs != 3 {
		return errors.Errorf("soft indexed %d documents, want 3", idx.Header.NumDocs)
	}

	fmt.Println(softSkippedTwoDocuments)
	fmt.Println(softIndexedThreeDocuments)
	fmt.Printf("%s %v\n", softDomainRowGroupsPrefix, result.ToSlice())
	return nil
}
