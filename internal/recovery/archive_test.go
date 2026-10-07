package recovery

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoundTripPreservesBinaryPhotosAndRejectsExistingTarget(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "house"), 0700); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(source, "house", "photo.webp"), data, 0600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	want, err := Pack(source, &archive)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadArchive(bytes.NewReader(archive.Bytes()), target, want.Files, want.Bytes)
	if err != nil || got != want {
		t.Fatalf("round trip: %v, %v", got, err)
	}
	restored, err := os.ReadFile(filepath.Join(target, "house", "photo.webp"))
	if err != nil || !bytes.Equal(restored, data) {
		t.Fatal("binary photo changed")
	}
	if _, err := ReadArchive(bytes.NewReader(archive.Bytes()), target, want.Files, want.Bytes); err == nil {
		t.Fatal("nonempty target accepted")
	}
	if _, err := ReadArchive(bytes.NewReader(archive.Bytes()), "", 0, want.Bytes); err == nil {
		t.Fatal("file bound ignored")
	}
	if _, err := ReadArchive(bytes.NewReader(archive.Bytes()), "", want.Files, want.Bytes-1); err == nil {
		t.Fatal("byte bound ignored")
	}
}
func TestArchiveRejectsTraversalLinksDuplicatesAndTruncation(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "C:/drive", "a/../escape", "a\\escape", "valid"} {
		for _, kind := range []byte{tar.TypeReg, tar.TypeSymlink, tar.TypeLink} {
			var archive bytes.Buffer
			tw := tar.NewWriter(&archive)
			header := &tar.Header{Name: name, Typeflag: kind, Linkname: "../outside"}
			if err := tw.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if kind == tar.TypeReg && name == "valid" {
				if err := tw.WriteHeader(header); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadArchive(bytes.NewReader(archive.Bytes()), t.TempDir(), 10, 1024); err == nil {
				t.Fatalf("unsafe entry accepted: %q, %d", name, kind)
			}
		}
	}
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := tw.WriteHeader(&tar.Header{Name: "photo", Typeflag: tar.TypeReg, Size: 512, Mode: 0600}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArchive(bytes.NewReader(archive.Bytes()), t.TempDir(), 1, 512); err == nil {
		t.Fatal("truncated image accepted")
	}
}
