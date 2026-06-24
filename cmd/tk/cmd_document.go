package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/libticket"
)

func runDocument(args []string) error {
	if len(args) == 0 {
		fmt.Println(documentUsage)
		return nil
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(documentUsage)
		return nil
	}
	cfg, _, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "create", "new", "add":
		fs := flag.NewFlagSet("document create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "document title")
		description := fs.String("d", "", "document description")
		notes := fs.String("notes", "", "document notes")
		content := fs.String("content", "", "document text content")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			return errors.New("usage: tk document create -title <title>\n" +
				"  [-d <description>] [-notes <notes>] [-content <text>]")
		}
		document, err := svc.CreateDocument(context.Background(), project.ID, libticket.DocumentRequest{
			Title:       *title,
			Description: *description,
			Notes:       *notes,
			Content:     *content,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(document)
		}
		fmt.Printf("document %d: %s\n", document.ID, document.Title)
		return nil
	case "ls", "list":
		documents, err := svc.ListDocuments(context.Background(), project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(documents)
		}
		if len(documents) == 0 {
			printNoEntitiesAvailable("documents")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tUPDATED")
		for _, doc := range documents {
			fmt.Fprintf(w, "%d\t%s\t%s\n", doc.ID, doc.Title, doc.UpdatedAt)
		}
		return w.Flush()
	case "get", "show":
		if len(args) != 2 {
			return errors.New("usage: tk document get <id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		doc, err := svc.GetDocument(context.Background(), documentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(doc)
		}
		fmt.Printf("ID          : %d\n", doc.ID)
		fmt.Printf("ProjectID   : %d\n", doc.ProjectID)
		fmt.Printf("Title       : %s\n", doc.Title)
		fmt.Printf("Description : %s\n", doc.Description)
		fmt.Printf("Notes       : %s\n", doc.Notes)
		fmt.Printf("Content     : %s\n", doc.Content)
		fmt.Printf("Created     : %s\n", doc.CreatedAt)
		fmt.Printf("Updated     : %s\n", doc.UpdatedAt)
		return nil
	case "update":
		if len(args) < 2 {
			return errors.New("usage: tk document update <id>\n" +
				"  [-title <title>] [-d <description>] [-notes <notes>] [-content <text>]")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		fs := flag.NewFlagSet("document update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "document title")
		description := fs.String("d", "", "document description")
		notes := fs.String("notes", "", "document notes")
		content := fs.String("content", "", "document text content")
		if parseErr := fs.Parse(args[2:]); parseErr != nil {
			return parseErr
		}
		current, err := svc.GetDocument(context.Background(), documentID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			*title = current.Title
		}
		if strings.TrimSpace(*description) == "" {
			*description = current.Description
		}
		if strings.TrimSpace(*notes) == "" {
			*notes = current.Notes
		}
		if *content == "" {
			*content = current.Content
		}
		updated, err := svc.UpdateDocument(context.Background(), documentID, libticket.DocumentRequest{
			Title:       *title,
			Description: *description,
			Notes:       *notes,
			Content:     *content,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(updated)
		}
		fmt.Printf("document %d updated: %s\n", updated.ID, updated.Title)
		return nil
	case "rm", "delete":
		if len(args) != 2 {
			return errors.New("usage: tk document rm <id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		if err := svc.DeleteDocument(context.Background(), documentID); err != nil {
			return err
		}
		fmt.Printf("deleted document %d\n", documentID)
		return nil
	case "label-add":
		if len(args) != 3 {
			return errors.New("usage: tk document label-add <document-id> <label-id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		labelID, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
		if err != nil || labelID <= 0 {
			return fmt.Errorf("invalid label id %q", args[2])
		}
		return svc.AddDocumentLabel(context.Background(), documentID, libticket.DocumentLabelRequest{LabelID: labelID})
	case "label-rm":
		if len(args) != 3 {
			return errors.New("usage: tk document label-rm <document-id> <label-id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		labelID, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
		if err != nil || labelID <= 0 {
			return fmt.Errorf("invalid label id %q", args[2])
		}
		return svc.RemoveDocumentLabel(context.Background(), documentID, labelID)
	case "label-ls":
		if len(args) != 2 {
			return errors.New("usage: tk document label-ls <document-id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		labels, err := svc.ListDocumentLabels(context.Background(), documentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(labels)
		}
		if len(labels) == 0 {
			printNoEntitiesAvailable("labels")
			return nil
		}
		for _, l := range labels {
			fmt.Printf("%d\t%s\n", l.ID, l.Name)
		}
		return nil
	case "file-add":
		fs := flag.NewFlagSet("document file-add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		path := fs.String("path", "", "path to file")
		name := fs.String("name", "", "file name override")
		contentType := fs.String("content-type", "", "content type override")
		if len(args) < 2 {
			return errors.New("usage: tk document file-add <document-id> -path <file>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		if parseErr := fs.Parse(args[2:]); parseErr != nil {
			return parseErr
		}
		if strings.TrimSpace(*path) == "" {
			return errors.New("file path is required")
		}
		content, err := os.ReadFile(*path)
		if err != nil {
			return err
		}
		fileName := strings.TrimSpace(*name)
		if fileName == "" {
			fileName = filepath.Base(*path)
		}
		if strings.TrimSpace(*contentType) == "" {
			*contentType = http.DetectContentType(content)
		}
		file, err := svc.AddDocumentFile(context.Background(), documentID, libticket.DocumentFileUploadRequest{
			FileName:    fileName,
			ContentType: *contentType,
			Content:     content,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(file)
		}
		fmt.Printf("document %d file %d added: %s\n", documentID, file.ID, file.FileName)
		return nil
	case "file-ls":
		if len(args) != 2 {
			return errors.New("usage: tk document file-ls <document-id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		files, err := svc.ListDocumentFiles(context.Background(), documentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(files)
		}
		if len(files) == 0 {
			printNoEntitiesAvailable("document files")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "FILE_ID\tNAME\tSIZE\tCONTENT_TYPE")
		for _, file := range files {
			fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", file.ID, file.FileName, file.SizeBytes, file.ContentType)
		}
		return w.Flush()
	case "file-get":
		fs := flag.NewFlagSet("document file-get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		outputPath := fs.String("o", "", "output path")
		if len(args) < 3 {
			return errors.New("usage: tk document file-get <document-id> <file-id> -o <path>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		fileID, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
		if err != nil || fileID <= 0 {
			return fmt.Errorf("invalid file id %q", args[2])
		}
		if parseErr := fs.Parse(args[3:]); parseErr != nil {
			return parseErr
		}
		if strings.TrimSpace(*outputPath) == "" {
			return errors.New("output path is required")
		}
		file, err := svc.GetDocumentFile(context.Background(), documentID, fileID)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outputPath, file.Content, 0o600); err != nil {
			return err
		}
		fmt.Printf("wrote %d bytes to %s\n", len(file.Content), *outputPath)
		return nil
	case "file-rm":
		if len(args) != 3 {
			return errors.New("usage: tk document file-rm <document-id> <file-id>")
		}
		documentID, err := parseDocumentID(args[1])
		if err != nil {
			return err
		}
		fileID, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
		if err != nil || fileID <= 0 {
			return fmt.Errorf("invalid file id %q", args[2])
		}
		return svc.DeleteDocumentFile(context.Background(), documentID, fileID)
	default:
		return fmt.Errorf("unknown document command %q; see: tk document help", args[0])
	}
}

func parseDocumentID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid document id %q", raw)
	}
	return id, nil
}
