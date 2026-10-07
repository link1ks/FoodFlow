// Recovery is an offline maintenance helper. It never prints credentials or rows.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"foodflow/internal/recovery"
	"io"
	"os"
	"reflect"
	"time"
)

func readJSON(v any) error {
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return errors.New("invalid private recovery input")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("unexpected recovery input")
	}
	return nil
}
func run() error {
	mode := flag.String("mode", "snapshot", "snapshot, archive, inspect, unpack, compare or probe")
	maxFiles := flag.Int64("files", 0, "manifest file bound")
	maxBytes := flag.Int64("bytes", 0, "manifest byte bound")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root := os.Getenv("IMAGE_STORAGE_DIR")
	if root == "" {
		root = "/app/data/images"
	}
	var result any
	switch *mode {
	case "snapshot":
		value, err := recovery.Capture(ctx, os.Getenv("DATABASE_URL"), root)
		if err != nil {
			return err
		}
		result = value
	case "archive":
		_, err := recovery.Pack(root, os.Stdout)
		return err
	case "inspect", "unpack":
		target := ""
		if *mode == "unpack" {
			target = root
		}
		value, err := recovery.ReadArchive(os.Stdin, target, *maxFiles, *maxBytes)
		if err != nil {
			return err
		}
		result = value
	case "compare":
		var want recovery.Snapshot
		if err := readJSON(&want); err != nil || want.Version != 1 {
			return errors.New("invalid expected snapshot")
		}
		actual, err := recovery.Capture(ctx, os.Getenv("DATABASE_URL"), root)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, want) {
			return errors.New("restored database or photos differ from backup snapshot")
		}
		result = map[string]any{"passed": true, "tables": len(actual.Tables), "image_files": actual.Images.Files, "referenced_images": actual.ReferencedImages}
	case "probe":
		value, err := probe(ctx)
		if err != nil {
			return err
		}
		result = value
	default:
		return errors.New("unknown recovery mode")
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	deadline := time.AfterFunc(5*time.Minute, func() {
		fmt.Fprintln(os.Stderr, "recovery deadline exceeded")
		os.Exit(1)
	})
	defer deadline.Stop()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
