package ticket

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blater/goname"
)

// ErrAlreadyClaimed is returned when trying to claim a ticket that is not open.
var ErrAlreadyClaimed = errors.New("ticket already claimed")

const (
	// TicketsDirName is the name of the tickets directory.
	TicketsDirName = ".tickets"
	// IDPrefix is the prefix for ticket IDs.
	IDPrefix = "tic"
	// TicketReadmeName is reserved for source-controlled store documentation.
	TicketReadmeName = "README.md"
)

const maxThreeWordAttempts = 3

// Store handles ticket file operations on a .tickets/ directory.
type Store struct {
	ticketsDir string
}

// Open opens a ticket store at dir/.tickets/.
func Open(dir string) *Store {
	return &Store{
		ticketsDir: filepath.Join(dir, TicketsDirName),
	}
}

// OpenDir opens a ticket store using an explicit .tickets/ directory path.
func OpenDir(ticketsDir string) *Store {
	return &Store{
		ticketsDir: ticketsDir,
	}
}

// FindTicketsDir finds the .tickets directory by walking up parent directories
// from the current working directory.
func FindTicketsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		ticketsPath := filepath.Join(dir, TicketsDirName)
		if info, err := os.Stat(ticketsPath); err == nil && info.IsDir() {
			return ticketsPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .tickets
			return "", fmt.Errorf("no %s directory found", TicketsDirName)
		}
		dir = parent
	}
}

// TicketsDir returns the tickets directory path.
func (s *Store) TicketsDir() string {
	return s.ticketsDir
}

// GenerateID generates a one-word ticket ID candidate. Use Store.GenerateID
// when the ID must be checked against tickets already in a store.
func GenerateID() (string, error) {
	name, err := generateTicketName(1)
	if err != nil {
		return "", err
	}
	return ticketID(name), nil
}

// GenerateID generates a ticket ID that does not collide with an ID currently
// in the store. It tries one word, then two words, then up to three different
// three-word names before falling back to a GUID.
func (s *Store) GenerateID() (string, error) {
	ids, err := s.ListIDs()
	if err != nil {
		return "", err
	}
	existing := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		existing[id] = struct{}{}
	}
	return generateUniqueID(existing, generateTicketName, generateGUID)
}

func generateUniqueID(existing map[string]struct{}, generateName func(int) (string, error), generateGUID func() (string, error)) (string, error) {
	return generateUniqueIDWithReservation(existing, generateName, generateGUID, nil)
}

func generateUniqueIDWithReservation(existing map[string]struct{}, generateName func(int) (string, error), generateGUID func() (string, error), reserve func(string) (bool, error)) (string, error) {
	try := func(id string) (bool, error) {
		if _, collision := existing[id]; collision {
			return false, nil
		}
		if reserve == nil {
			return true, nil
		}
		return reserve(id)
	}

	for _, words := range []int{1, 2} {
		name, err := generateName(words)
		if err != nil {
			return "", err
		}
		id := ticketID(name)
		if reserved, err := try(id); err != nil {
			return "", err
		} else if reserved {
			return id, nil
		}
	}

	for range maxThreeWordAttempts {
		name, err := generateName(3)
		if err != nil {
			return "", err
		}
		id := ticketID(name)
		if reserved, err := try(id); err != nil {
			return "", err
		} else if reserved {
			return id, nil
		}
	}

	id, err := generateGUID()
	if err != nil {
		return "", err
	}
	if reserved, err := try(id); err != nil {
		return "", err
	} else if !reserved {
		return "", fmt.Errorf("generated GUID ticket ID already exists: %s", id)
	}
	return id, nil
}

func generateTicketName(words int) (string, error) {
	options := goname.DefaultOptions()
	options.Words = words
	options.Strategy = goname.StrategyTolkien
	name, err := goname.GenerateWithOptions(options)
	if err != nil {
		return "", fmt.Errorf("failed to generate ticket name: %w", err)
	}
	return strings.ToLower(name), nil
}

func ticketID(name string) string {
	return fmt.Sprintf("%s-%s", IDPrefix, name)
}

func generateGUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate GUID: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	guid := fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:])
	return ticketID(guid), nil
}

// Create writes a new ticket without overwriting an existing ticket file. If
// ticket.ID is empty, it generates a unique name and advances through the
// configured collision tiers until it can reserve the file.
func (s *Store) Create(ticket *Ticket) error {
	if err := s.EnsureDir(); err != nil {
		return fmt.Errorf("failed to create tickets directory: %w", err)
	}
	if ticket.ID != "" {
		return s.writeNew(ticket)
	}

	ids, err := s.ListIDs()
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		existing[id] = struct{}{}
	}
	_, err = generateUniqueIDWithReservation(existing, generateTicketName, generateGUID, func(id string) (bool, error) {
		ticket.ID = id
		if err := s.writeNew(ticket); err != nil {
			if errors.Is(err, os.ErrExist) {
				existing[id] = struct{}{}
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}

func (s *Store) writeNew(ticket *Ticket) error {
	data, err := ticket.Render()
	if err != nil {
		return err
	}
	path := filepath.Join(s.ticketsDir, ticket.ID+".md")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to create ticket file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("failed to write ticket file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("failed to close ticket file: %w", err)
	}
	return nil
}

// List returns all tickets in the storage directory.
func (s *Store) List() ([]*Ticket, error) {
	entries, err := os.ReadDir(s.ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read tickets directory: %w", err)
	}

	var tickets []*Ticket
	for _, entry := range entries {
		if !isTicketFile(entry) {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".md")
		ticket, err := s.Read(id)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// Read reads a ticket by ID.
func (s *Store) Read(id string) (*Ticket, error) {
	path := filepath.Join(s.ticketsDir, id+".md")
	return ParseFromFile(path)
}

// Write saves a ticket to storage.
func (s *Store) Write(ticket *Ticket) error {
	path := filepath.Join(s.ticketsDir, ticket.ID+".md")
	return ticket.WriteToFile(path)
}

// Delete removes a ticket from storage.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.ticketsDir, id+".md")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete ticket %s: %w", id, err)
	}
	return nil
}

// Exists checks if a ticket exists.
func (s *Store) Exists(id string) bool {
	path := filepath.Join(s.ticketsDir, id+".md")
	_, err := os.Stat(path)
	return err == nil
}

// ResolveID resolves a partial ID to a full ticket ID.
// Returns the full ID if exactly one match is found.
// Returns an error if no match or multiple matches are found.
func (s *Store) ResolveID(partial string) (string, error) {
	entries, err := os.ReadDir(s.ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("ticket not found: %s", partial)
		}
		return "", fmt.Errorf("failed to read tickets directory: %w", err)
	}

	var matches []string
	for _, entry := range entries {
		if !isTicketFile(entry) {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".md")
		if strings.Contains(id, partial) {
			matches = append(matches, id)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("ticket not found: %s", partial)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous ID %s matches: %s", partial, strings.Join(matches, ", "))
	}
}

// ListIDs returns all ticket IDs.
func (s *Store) ListIDs() ([]string, error) {
	entries, err := os.ReadDir(s.ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read tickets directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if !isTicketFile(entry) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".md"))
	}

	return ids, nil
}

func isTicketFile(entry os.DirEntry) bool {
	return !entry.IsDir() && entry.Name() != TicketReadmeName && filepath.Ext(entry.Name()) == ".md"
}

// EnsureDir ensures the tickets directory exists.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.ticketsDir, 0755)
}

// SetExternalRef sets the external-ref field on a ticket. An empty ref clears it.
// Returns the updated ticket.
func (s *Store) SetExternalRef(id, ref string) (*Ticket, error) {
	ticket, err := s.Read(id)
	if err != nil {
		return nil, err
	}

	ticket.ExternalRef = ref

	if err := s.Write(ticket); err != nil {
		return nil, fmt.Errorf("failed to write ticket: %w", err)
	}

	return ticket, nil
}

// SetPR sets the pr field on a ticket (the pull/merge request reference).
// An empty ref clears it. Returns the updated ticket.
func (s *Store) SetPR(id, ref string) (*Ticket, error) {
	ticket, err := s.Read(id)
	if err != nil {
		return nil, err
	}

	ticket.PR = ref

	if err := s.Write(ticket); err != nil {
		return nil, fmt.Errorf("failed to write ticket: %w", err)
	}

	return ticket, nil
}

// AtomicClaim atomically claims a ticket by acquiring an exclusive file lock,
// checking the current status, and updating to in_progress only if the ticket is open.
// Returns ErrAlreadyClaimed if the ticket is not in open status.
func (s *Store) AtomicClaim(id string) (*Ticket, error) {
	return s.AtomicClaimWith(id, nil)
}

// AtomicClaimWith atomically claims a ticket and applies prepare while the
// ticket file is locked. prepare may attach branch and base-revision metadata
// derived before the claim.
func (s *Store) AtomicClaimWith(id string, prepare func(*Ticket) error) (*Ticket, error) {
	path := filepath.Join(s.ticketsDir, id+".md")

	// Open file for read/write
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open ticket file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Acquire exclusive lock (blocking)
	if err := lockFile(file); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = unlockFile(file) }()

	// Read current content
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ticket: %w", err)
	}

	ticket, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ticket: %w", err)
	}

	// Check if claimable
	if ticket.Status != StatusOpen {
		return nil, fmt.Errorf("%w: status is %s", ErrAlreadyClaimed, ticket.Status)
	}
	if prepare != nil {
		if err := prepare(ticket); err != nil {
			return nil, fmt.Errorf("failed to prepare claim: %w", err)
		}
	}

	// Update status
	ticket.Status = StatusInProgress

	// Write back (truncate and write)
	newData, err := ticket.Render()
	if err != nil {
		return nil, fmt.Errorf("failed to render ticket: %w", err)
	}

	if err := file.Truncate(0); err != nil {
		return nil, fmt.Errorf("failed to truncate file: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}
	if _, err := file.Write(newData); err != nil {
		return nil, fmt.Errorf("failed to write ticket: %w", err)
	}

	return ticket, nil
}
