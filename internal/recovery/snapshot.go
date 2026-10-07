package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

type Table struct {
	Name   string `json:"name"`
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}
type Sequence struct {
	Name      string `json:"name"`
	LastValue int64  `json:"last_value"`
	IsCalled  bool   `json:"is_called"`
}
type Snapshot struct {
	Version          int        `json:"version"`
	PostgresMajor    int        `json:"postgres_major"`
	Tables           []Table    `json:"tables"`
	Sequences        []Sequence `json:"sequences"`
	Images           Images     `json:"images"`
	ReferencedImages int64      `json:"referenced_images"`
}

func Capture(ctx context.Context, databaseURL, imageRoot string) (Snapshot, error) {
	result := Snapshot{Version: 1, Tables: []Table{}, Sequences: []Sequence{}}
	db, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return result, errors.New("backup database connection unavailable")
	}
	defer db.Close(ctx)
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, errors.New("cannot start read-only snapshot")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SET LOCAL TIME ZONE 'UTC'"); err != nil {
		return result, errors.New("cannot normalize snapshot timezone")
	}
	if err = tx.QueryRow(ctx, "SELECT current_setting('server_version_num')::int / 10000").Scan(&result.PostgresMajor); err != nil {
		return result, errors.New("cannot read PostgreSQL version")
	}
	rows, err := tx.Query(ctx, "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='public' ORDER BY tablename COLLATE \"C\"")
	if err != nil {
		return result, errors.New("cannot enumerate kitchen tables")
	}
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return result, errors.New("invalid table metadata")
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(names) == 0 {
		return result, errors.New("kitchen schema is missing")
	}
	for _, name := range names {
		table := Table{Name: name}
		h := sha256.New()
		query := "SELECT row_to_json(t)::text FROM " + (pgx.Identifier{"public", name}).Sanitize() + " t ORDER BY row_to_json(t)::text COLLATE \"C\""
		data, queryErr := tx.Query(ctx, query)
		if queryErr != nil {
			return result, errors.New("cannot read table fingerprint")
		}
		for data.Next() {
			var raw string
			if err = data.Scan(&raw); err != nil {
				data.Close()
				return result, errors.New("cannot decode table fingerprint")
			}
			fmt.Fprintf(h, "%d\x00%s\n", len(raw), raw)
			table.Rows++
		}
		err = data.Err()
		data.Close()
		if err != nil {
			return result, errors.New("incomplete table fingerprint")
		}
		table.SHA256 = hex.EncodeToString(h.Sum(nil))
		result.Tables = append(result.Tables, table)
	}
	sequences, err := tx.Query(ctx, `SELECT sequencename FROM pg_catalog.pg_sequences WHERE schemaname='public' ORDER BY sequencename COLLATE "C"`)
	if err != nil {
		return result, errors.New("cannot enumerate kitchen sequences")
	}
	var sequenceNames []string
	for sequences.Next() {
		var name string
		if err = sequences.Scan(&name); err != nil {
			sequences.Close()
			return result, errors.New("invalid sequence metadata")
		}
		sequenceNames = append(sequenceNames, name)
	}
	err = sequences.Err()
	sequences.Close()
	if err != nil {
		return result, errors.New("incomplete sequence metadata")
	}
	for _, name := range sequenceNames {
		value := Sequence{Name: name}
		if err = tx.QueryRow(ctx, "SELECT last_value, is_called FROM "+(pgx.Identifier{"public", name}).Sanitize()).Scan(&value.LastValue, &value.IsCalled); err != nil {
			return result, errors.New("cannot read sequence state")
		}
		result.Sequences = append(result.Sequences, value)
	}
	refs, err := tx.Query(ctx, `SELECT image_key FROM ingredients WHERE image_key IS NOT NULL
 UNION SELECT payload->>'key' FROM jobs WHERE kind='image' AND status IN ('queued','running','awaiting_confirmation')`)
	if err != nil {
		return result, errors.New("cannot read current photo references")
	}
	for refs.Next() {
		var key string
		if err = refs.Scan(&key); err != nil || !safeName(key) {
			refs.Close()
			return result, errors.New("invalid current photo reference")
		}
		info, statErr := os.Lstat(filepath.Join(imageRoot, filepath.FromSlash(key)))
		if statErr != nil || !info.Mode().IsRegular() {
			refs.Close()
			return result, errors.New("a referenced photo is missing or unsafe")
		}
		result.ReferencedImages++
	}
	err = refs.Err()
	refs.Close()
	if err != nil {
		return result, errors.New("incomplete photo references")
	}
	result.Images, err = Pack(imageRoot, nil)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, errors.New("cannot complete read-only snapshot")
	}
	return result, nil
}
