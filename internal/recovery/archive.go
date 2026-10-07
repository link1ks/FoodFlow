// Package recovery verifies private kitchen backups without mutating source data.
package recovery

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Images struct {
	Files  int64  `json:"files"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type imageFact struct {
	name, sum string
	size      int64
}

func safeName(name string) bool {
	return name != "" && name != "." && len(name) <= 4096 && path.Clean(name) == name &&
		!strings.HasPrefix(name, "/") && !strings.ContainsAny(name, "\\:\x00") && name != ".." && !strings.HasPrefix(name, "../")
}
func imageDigest(facts []imageFact) Images {
	sort.Slice(facts, func(i, j int) bool { return facts[i].name < facts[j].name })
	h := sha256.New()
	r := Images{Files: int64(len(facts))}
	for _, f := range facts {
		fmt.Fprintf(h, "%s\x00%d\x00%s\n", f.name, f.size, f.sum)
		r.Bytes += f.size
	}
	r.SHA256 = hex.EncodeToString(h.Sum(nil))
	return r
}

// Paths appear only inside the private archive. The summary has no household IDs.
func Pack(root string, dst io.Writer) (Images, error) {
	var facts []imageFact
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Images{}, errors.New("image root must be a real directory")
	}
	var tw *tar.Writer
	if dst != nil {
		tw = tar.NewWriter(dst)
		defer tw.Close()
	}
	err = filepath.WalkDir(root, func(file string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("cannot read image tree")
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("image tree contains a link or special file")
		}
		rel, err := filepath.Rel(root, file)
		name := filepath.ToSlash(rel)
		if err != nil || !safeName(name) {
			return errors.New("unsafe image path")
		}
		f, err := os.Open(file)
		if err != nil {
			return errors.New("cannot open image file")
		}
		defer f.Close()
		h := sha256.New()
		var output io.Writer = h
		if tw != nil {
			if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			output = io.MultiWriter(h, tw)
		}
		n, err := io.Copy(output, f)
		if err != nil || n != info.Size() {
			return errors.New("image changed while reading")
		}
		facts = append(facts, imageFact{name, hex.EncodeToString(h.Sum(nil)), n})
		return nil
	})
	if err == nil && tw != nil {
		err = tw.Close()
	}
	return imageDigest(facts), err
}

// Extraction accepts only regular files in an empty real directory. Failed
// targets are retained; nothing is overwritten or cleared.
func ReadArchive(src io.Reader, root string, maxFiles, maxBytes int64) (Images, error) {
	if maxFiles < 0 || maxBytes < 0 {
		return Images{}, errors.New("invalid archive bounds")
	}
	if root != "" {
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return Images{}, errors.New("restore root must be a real directory")
		}
		files, err := os.ReadDir(root)
		if err != nil || len(files) != 0 {
			return Images{}, errors.New("restore image target must be empty")
		}
	}
	seen := map[string]bool{}
	var facts []imageFact
	var total int64
	tr := tar.NewReader(src)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Images{}, errors.New("invalid image archive")
		}
		if !safeName(header.Name) || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || seen[header.Name] {
			return Images{}, errors.New("unsafe, special or duplicate archive entry")
		}
		if header.Size < 0 || int64(len(facts)) >= maxFiles || header.Size > maxBytes-total {
			return Images{}, errors.New("image archive exceeds manifest bounds")
		}
		seen[header.Name] = true
		h := sha256.New()
		var dst io.Writer = h
		var f *os.File
		if root != "" {
			file := filepath.Join(root, filepath.FromSlash(header.Name))
			if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				return Images{}, errors.New("cannot prepare image target")
			}
			f, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return Images{}, errors.New("cannot create new image target")
			}
			dst = io.MultiWriter(h, f)
		}
		n, copyErr := io.Copy(dst, tr)
		if f != nil {
			if err = f.Close(); copyErr == nil {
				copyErr = err
			}
		}
		if copyErr != nil || n != header.Size {
			return Images{}, errors.New("incomplete image archive entry")
		}
		total += n
		facts = append(facts, imageFact{header.Name, hex.EncodeToString(h.Sum(nil)), n})
	}
	return imageDigest(facts), nil
}
