package sleepy

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Device struct {
	ID          string         `json:"id"`
	ShowName    string         `json:"show_name"`
	Using       *bool          `json:"using"`
	Status      *string        `json:"status"`
	Fields      map[string]any `json:"fields"`
	LastUpdated float64        `json:"last_updated"`
}
type Snapshot struct {
	Status  int
	Private bool
	Updated float64
	Devices map[string]Device
}
type Store struct {
	mu      sync.RWMutex
	db      *sql.DB
	state   Snapshot
	changed chan struct{}
}

func nowSeconds() float64 { return float64(time.Now().UnixMicro()) / 1e6 }
func openStore(c Config) (*Store, error) {
	filename := c.Main.Database
	switch {
	case filename == "sqlite:///../data/data.db":
		filename = filepath.Join(c.DataDir, "data.db")
	case filename == "sqlite:///:memory:":
		filename = ":memory:"
	case strings.HasPrefix(filename, "sqlite:///"):
		filename = strings.TrimPrefix(filename, "sqlite:///")
		if !filepath.IsAbs(filename) {
			filename = filepath.Join("instance", filename)
		}
	default:
		return nil, fmt.Errorf("Go server currently supports SQLite databases only")
	}
	if filename != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			return nil, err
		}
		if err := checkWritable(filename); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(e error) (*Store, error) { db.Close(); return nil, e }
	for _, q := range []string{
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS main (id INTEGER PRIMARY KEY, status INTEGER NOT NULL DEFAULT 0, private_mode BOOLEAN NOT NULL DEFAULT 0, last_updated FLOAT)`,
		`CREATE TABLE IF NOT EXISTS device_status (id VARCHAR(1024) PRIMARY KEY, show_name VARCHAR(1024) NOT NULL, "using" BOOLEAN, status TEXT, fields JSON NOT NULL DEFAULT '{}', last_updated FLOAT)`,
	} {
		if _, err = db.Exec(q); err != nil {
			return fail(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO main (id,status,private_mode,last_updated) SELECT 0,0,0,? WHERE NOT EXISTS (SELECT 1 FROM main)`, nowSeconds()); err != nil {
		return fail(err)
	}
	s := &Store{db: db, changed: make(chan struct{})}
	if err = s.reload(); err != nil {
		return fail(err)
	}
	return s, nil
}

func checkWritable(filename string) error {
	hint := func(path string, err error) error {
		return fmt.Errorf("%s is not writable by uid %d: %w (Docker: run the container as the owner of data/, e.g. user: \"0:0\", or chown data/ to the container user)", path, os.Getuid(), err)
	}
	dir := filepath.Dir(filename)
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return hint(dir, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	if file, err := os.OpenFile(filename, os.O_RDWR, 0); err == nil {
		file.Close()
	} else if !os.IsNotExist(err) {
		return hint(filename, err)
	}
	return nil
}

func (s *Store) reload() error {
	state := Snapshot{Devices: map[string]Device{}}
	if err := s.db.QueryRow(`SELECT status,private_mode,COALESCE(last_updated,0) FROM main ORDER BY id LIMIT 1`).Scan(&state.Status, &state.Private, &state.Updated); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT id,show_name,"using",status,COALESCE(fields,'{}'),COALESCE(last_updated,0) FROM device_status ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d Device
		var fields string
		if err = rows.Scan(&d.ID, &d.ShowName, &d.Using, &d.Status, &fields, &d.LastUpdated); err != nil {
			return err
		}
		if d.Fields, err = decodeFields(fields); err != nil { // 1 junk row != whole svc down
			log.Printf("device %q: ignoring unreadable fields: %v", d.ID, err)
			d.Fields = map[string]any{}
		}
		state.Devices[d.ID] = d
	}
	if err = rows.Err(); err != nil {
		return err
	}
	s.state = state
	return nil
}

func (s *Store) snapshot() (Snapshot, <-chan struct{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.changed
}
func (s *Store) write(fn func(*sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		if errors.Is(err, errNoChange) {
			return nil
		}
		return err
	}
	if _, err = tx.Exec(`UPDATE main SET last_updated=? WHERE id=(SELECT id FROM main ORDER BY id LIMIT 1)`, nowSeconds()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = s.reload(); err != nil {
		return fmt.Errorf("change saved, but reloading state failed: %w", err)
	}
	close(s.changed)
	s.changed = make(chan struct{})
	return nil
}
func (s *Store) setStatus(id int) error {
	return s.write(func(tx *sql.Tx) error {
		if s.state.Status == id {
			return errNoChange
		}
		_, e := tx.Exec(`UPDATE main SET status=? WHERE id=(SELECT id FROM main ORDER BY id LIMIT 1)`, id)
		return e
	})
}
func (s *Store) setPrivate(value bool) error {
	return s.write(func(tx *sql.Tx) error {
		if s.state.Private == value {
			return errNoChange
		}
		_, e := tx.Exec(`UPDATE main SET private_mode=? WHERE id=(SELECT id FROM main ORDER BY id LIMIT 1)`, value)
		return e
	})
}
func (s *Store) deleteDevice(id string, all bool) error {
	return s.write(func(tx *sql.Tx) error {
		if all {
			_, err := tx.Exec(`DELETE FROM device_status`)
			return err
		}
		_, e := tx.Exec(`DELETE FROM device_status WHERE id=?`, id)
		return e
	})
}

type DeviceUpdate struct {
	ID         string         `json:"id"`
	ShowName   *string        `json:"show_name"`
	Using      *bool          `json:"using"`
	Status     *string        `json:"status"`
	AppName    *string        `json:"app_name"`
	Fields     map[string]any `json:"fields"`
	reviveFrom *int
}

func (s *Store) setDevice(u DeviceUpdate) error {
	return s.write(func(tx *sql.Tx) error {
		var name string
		var using *bool
		var status *string
		var raw string
		err := tx.QueryRow(`SELECT show_name,"using",status,COALESCE(fields,'{}') FROM device_status WHERE id=?`, u.ID).Scan(&name, &using, &status, &raw)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows && (u.ShowName == nil || *u.ShowName == "") {
			return apiError{400, "device show_name cannot be empty!"}
		}
		fields, e := decodeFields(raw)
		if e != nil {
			log.Printf("device %q: replacing unreadable fields: %v", u.ID, e)
			fields = map[string]any{}
		}
		if u.ShowName != nil {
			if *u.ShowName == "" {
				return apiError{400, "device show_name cannot be empty!"}
			}
			name = *u.ShowName
		}
		if u.Using != nil {
			using = u.Using
		}
		if u.Status != nil {
			status = u.Status
		} else if u.AppName != nil {
			status = u.AppName
		}
		mergeMap(fields, u.Fields)
		data, e := json.Marshal(fields)
		if e != nil {
			return e
		}
		_, e = tx.Exec(`INSERT INTO device_status (id,show_name,"using",status,fields,last_updated) VALUES (?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET show_name=excluded.show_name,"using"=excluded."using",status=excluded.status,fields=excluded.fields,last_updated=excluded.last_updated`, u.ID, name, using, status, string(data), nowSeconds())
		if e == nil && u.reviveFrom != nil {
			_, e = tx.Exec(`UPDATE main SET status=0 WHERE id=(SELECT id FROM main ORDER BY id LIMIT 1) AND status=?`, *u.reviveFrom)
		}
		return e
	})
}

var errNoChange = errors.New("no change")

// UseNumber or snowflake ids > 2^53 get float64'd
func decodeFields(raw string) (map[string]any, error) {
	fields := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return fields, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&fields); err != nil {
		return map[string]any{}, err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields, nil
}

func (s *Store) expireStatus(cutoff float64, dead int) error {
	return s.write(func(tx *sql.Tx) error {
		if s.state.Status == dead || s.state.Updated >= cutoff {
			return errNoChange
		}
		for _, d := range s.state.Devices {
			if d.LastUpdated >= cutoff {
				return errNoChange
			}
		}
		_, err := tx.Exec(`UPDATE main SET status=? WHERE id=(SELECT id FROM main ORDER BY id LIMIT 1)`, dead)
		return err
	})
}
