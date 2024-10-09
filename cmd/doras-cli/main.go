package main

import (
	"flag"
	"github.com/unbasical/doras-server/pkg/delta"
	"github.com/unbasical/doras-server/pkg/differ"
	"github.com/unbasical/doras-server/pkg/storage"
	"log"
	"os"
)

func main() {
	var (
		from      string
		to        string
		output    string
		input     string
		algorithm string
	)

	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	createCmd.StringVar(&from, "from", "", "path to the old file")
	createCmd.StringVar(&to, "to", "", "path to the old file")
	createCmd.StringVar(&algorithm, "algorithm", "bsdiff", "diffing algorithm")
	createCmd.StringVar(&output, "output", "", "path to the output file")
	applyCmd := flag.NewFlagSet("apply", flag.ExitOnError)
	applyCmd.StringVar(&input, "input", "", "path to the output file")
	applyCmd.StringVar(&output, "output", "", "path to the output file")
	applyCmd.StringVar(&from, "from", "", "path to the old file")
	applyCmd.StringVar(&algorithm, "algorithm", "bsdiff", "diffing algorithm")

	flag.Parse()
	s := storage.FilesystemStorage{BasePath: ""}
	var dif differ.Differ
	switch algorithm {
	case "bsdiff":
		dif = differ.Bsdiff{}
	default:
		log.Fatalf("unknown algorithm: %s", algorithm)
	}

	switch os.Args[1] {
	case "create":
		err := createCmd.Parse(os.Args[2:])

		if err != nil {
			log.Fatal(err)
		}
		if from == "" {
			log.Fatalf("--from is required")
		}
		if to == "" {
			log.Fatalf("--to is required")
		}
		if output == "" {
			log.Fatalf("--output is required")
		}
		fromArtifact, err := s.LoadArtifact(from)
		if err != nil {
			log.Fatal(err)
		}
		toArtifact, err := s.LoadArtifact(to)
		if err != nil {
			log.Fatal(err)
		}

		patch := dif.CreateDiff(fromArtifact, toArtifact)
		err = s.StoreDelta(delta.RawDiff{Data: patch}, output)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s patch written to %s", algorithm, output)
	case "apply":
		err := applyCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}
		if from == "" {
			log.Fatalf("--from is required")
		}
		if input == "" {
			log.Fatalf("--input is required")
		}
		if output == "" {
			log.Fatalf("--output is required")
		}
		fromArtifact, err := s.LoadArtifact(from)
		if err != nil {
			log.Fatal(err)
		}
		patch, err := s.LoadDelta(input)
		if err != nil {
			log.Fatal(err)
		}
		patchBytes, _ := patch.GetBytes()
		to := dif.ApplyDiff(fromArtifact, patchBytes)
		err = s.StoreArtifact(to, output)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s patched file written to %s", algorithm, output)
	}

}
