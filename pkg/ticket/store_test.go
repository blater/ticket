package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type StoreSuite struct {
	suite.Suite
	tempDir string
	store   *Store
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreSuite))
}

func (s *StoreSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "ticket-store-test-*")
	require.NoError(s.T(), err)

	ticketsDir := filepath.Join(s.tempDir, TicketsDirName)
	require.NoError(s.T(), os.MkdirAll(ticketsDir, 0755))

	s.store = OpenDir(ticketsDir)
}

func (s *StoreSuite) TearDownTest() {
	_ = os.RemoveAll(s.tempDir)
}

func (s *StoreSuite) TestOpen() {
	store := Open(s.tempDir)
	require.Equal(s.T(), filepath.Join(s.tempDir, TicketsDirName), store.TicketsDir())
}

func (s *StoreSuite) TestGenerateID() {
	id, err := GenerateID()
	require.NoError(s.T(), err)
	require.Regexp(s.T(), `^tic-[0-9a-f]{32}$`, id)
}

func (s *StoreSuite) TestGenerateIDWithTolkienStrategy() {
	id, err := GenerateIDWithStrategy(IDStrategyTolkien)
	require.NoError(s.T(), err)
	require.True(s.T(), strings.HasPrefix(id, IDPrefix+"-"))
	require.NotRegexp(s.T(), `^tic-[0-9a-f]{32}$`, id)
}

func (s *StoreSuite) TestGenerateIDWithGonameStrategies() {
	tests := []struct {
		strategy IDStrategy
		pattern  string
	}{
		{IDStrategyDefault, `^tic-[a-z]+$`},
		{IDStrategyTolkien, `^tic-[a-z]+$`},
		{IDStrategyHex, `^tic-[0-9a-f]{32}$`},
		{IDStrategyBase32, `^tic-[0-9a-hjkmnp-tv-z]{26}$`},
		{IDStrategyULID, `^tic-[0-9a-hjkmnp-tv-z]{26}$`},
	}
	for _, test := range tests {
		s.Run(string(test.strategy), func() {
			id, err := GenerateIDWithStrategy(test.strategy)
			require.NoError(s.T(), err)
			require.Regexp(s.T(), test.pattern, id)
		})
	}
}

func (s *StoreSuite) TestGenerateIDDefaultStrategyUsesGonameWords() {
	id, err := GenerateIDWithStrategy(IDStrategyDefault)
	require.NoError(s.T(), err)
	require.Regexp(s.T(), `^tic-[a-z]+$`, id)
}

func (s *StoreSuite) TestGenerateIDRejectsUnknownStrategy() {
	_, err := GenerateIDWithStrategy(IDStrategy("random"))
	require.ErrorContains(s.T(), err, "valid values: default|tolkien|hex|base32|ulid")
	_, err = GenerateIDWithStrategy(IDStrategy("guid"))
	require.ErrorContains(s.T(), err, "valid values: default|tolkien|hex|base32|ulid")
}

func (s *StoreSuite) TestGenerateIDUnique() {
	ids := make(map[string]bool)
	for range 10 {
		id, err := s.store.GenerateID()
		require.NoError(s.T(), err)
		require.False(s.T(), ids[id], "duplicate ID generated: %s", id)
		ids[id] = true
		require.NoError(s.T(), s.store.Write(&Ticket{ID: id, Status: StatusOpen, Created: time.Now().UTC()}))
	}
}

func TestGenerateUniqueIDEscalatesWordCount(t *testing.T) {
	var calls []int
	got, err := generateUniqueID(
		map[string]struct{}{"tic-otter": {}, "tic-bright-otter": {}},
		func(words int) (string, error) {
			calls = append(calls, words)
			switch words {
			case 1:
				return "tic-otter", nil
			case 2:
				return "tic-bright-otter", nil
			default:
				return "tic-calm-bright-otter", nil
			}
		},
		func() (string, error) { return "tic-fallback-hex", nil },
	)
	require.NoError(t, err)
	require.Equal(t, "tic-calm-bright-otter", got)
	require.Equal(t, []int{1, 2, 3}, calls)
}

func TestGenerateUniqueIDAdvancesWhenReservationCollides(t *testing.T) {
	var calls []int
	var reserved []string
	got, err := generateUniqueIDWithReservation(nil,
		func(words int) (string, error) {
			calls = append(calls, words)
			if words == 1 {
				return "tic-otter", nil
			}
			return "tic-bright-otter", nil
		},
		func() (string, error) { return "tic-fallback-hex", nil },
		func(id string) (bool, error) {
			reserved = append(reserved, id)
			return id != "tic-otter", nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "tic-bright-otter", got)
	require.Equal(t, []int{1, 2}, calls)
	require.Equal(t, []string{"tic-otter", "tic-bright-otter"}, reserved)
}

func TestGenerateUniqueIDFallsBackAfterThreeWordCollisions(t *testing.T) {
	var calls []int
	threeWordIDs := []string{"tic-quick-blue-owl", "tic-fast-blue-owl", "tic-calm-blue-owl"}
	existing := map[string]struct{}{
		"tic-owl":            {},
		"tic-blue-owl":       {},
		"tic-quick-blue-owl": {},
		"tic-fast-blue-owl":  {},
		"tic-calm-blue-owl":  {},
	}
	threeWordIndex := 0
	fallbackCalled := false
	got, err := generateUniqueID(existing,
		func(words int) (string, error) {
			calls = append(calls, words)
			switch words {
			case 1:
				return "tic-owl", nil
			case 2:
				return "tic-blue-owl", nil
			default:
				name := threeWordIDs[threeWordIndex]
				threeWordIndex++
				return name, nil
			}
		},
		func() (string, error) {
			fallbackCalled = true
			return "tic-fallback-hex", nil
		},
	)
	require.NoError(t, err)
	require.True(t, fallbackCalled)
	require.Equal(t, "tic-fallback-hex", got)
	require.Equal(t, []int{1, 2, 3, 3, 3}, calls)
}

func (s *StoreSuite) TestWriteAndRead() {
	ticket := &Ticket{
		ID:          "tic-test",
		Status:      StatusOpen,
		Type:        TypeTask,
		Priority:    2,
		Assignee:    "Developer",
		Created:     time.Now().UTC().Truncate(time.Second),
		Title:       "Test Ticket",
		Description: "Test description",
	}

	err := s.store.Write(ticket)
	require.NoError(s.T(), err)

	read, err := s.store.Read("tic-test")
	require.NoError(s.T(), err)
	require.Equal(s.T(), ticket.ID, read.ID)
	require.Equal(s.T(), ticket.Status, read.Status)
	require.Equal(s.T(), ticket.Type, read.Type)
	require.Equal(s.T(), ticket.Priority, read.Priority)
	require.Equal(s.T(), ticket.Assignee, read.Assignee)
	require.Equal(s.T(), ticket.Title, read.Title)
}

func (s *StoreSuite) TestCreateGeneratesAndStoresUniqueID() {
	first := &Ticket{Status: StatusOpen, Title: "First", Created: time.Now().UTC()}
	second := &Ticket{Status: StatusOpen, Title: "Second", Created: time.Now().UTC()}
	require.NoError(s.T(), s.store.Create(first))
	require.NoError(s.T(), s.store.Create(second))
	require.NotEmpty(s.T(), first.ID)
	require.NotEqual(s.T(), first.ID, second.ID)
	require.True(s.T(), s.store.Exists(first.ID))
	require.True(s.T(), s.store.Exists(second.ID))
	require.Regexp(s.T(), `^tic-[0-9a-f]{32}$`, first.ID)
}

func (s *StoreSuite) TestCreateWithTolkienStrategy() {
	ticket := &Ticket{Status: StatusOpen, Title: "Tolkien", Created: time.Now().UTC()}
	require.NoError(s.T(), s.store.CreateWithStrategy(ticket, IDStrategyTolkien))
	require.True(s.T(), strings.HasPrefix(ticket.ID, IDPrefix+"-"))
	require.NotRegexp(s.T(), `^tic-[0-9a-f]{32}$`, ticket.ID)
}

func (s *StoreSuite) TestCreateWithBase32AndULIDStrategies() {
	for _, test := range []struct {
		strategy IDStrategy
		pattern  string
	}{
		{IDStrategyBase32, `^tic-[0-9a-hjkmnp-tv-z]{26}$`},
		{IDStrategyULID, `^tic-[0-9a-hjkmnp-tv-z]{26}$`},
	} {
		s.Run(string(test.strategy), func() {
			ticket := &Ticket{Status: StatusOpen, Title: string(test.strategy), Created: time.Now().UTC()}
			require.NoError(s.T(), s.store.CreateWithStrategy(ticket, test.strategy))
			require.Regexp(s.T(), test.pattern, ticket.ID)
		})
	}
}

func (s *StoreSuite) TestCreateDoesNotOverwriteExistingID() {
	existing := &Ticket{ID: "tic-existing", Status: StatusOpen, Title: "Keep me", Created: time.Now().UTC()}
	require.NoError(s.T(), s.store.Write(existing))

	duplicate := &Ticket{ID: existing.ID, Status: StatusOpen, Title: "Overwrite", Created: time.Now().UTC()}
	err := s.store.Create(duplicate)
	require.ErrorIs(s.T(), err, os.ErrExist)

	read, err := s.store.Read(existing.ID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "Keep me", read.Title)
}

func (s *StoreSuite) TestList() {
	tickets := []*Ticket{
		{ID: "tic-aaa1", Status: StatusOpen, Created: time.Now().UTC()},
		{ID: "tic-bbb2", Status: StatusClosed, Created: time.Now().UTC()},
		{ID: "tic-ccc3", Status: StatusInProgress, Created: time.Now().UTC()},
	}

	for _, t := range tickets {
		require.NoError(s.T(), s.store.Write(t))
	}

	list, err := s.store.List()
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 3)
}

func (s *StoreSuite) TestList_EmptyDirectory() {
	list, err := s.store.List()
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 0)
}

func (s *StoreSuite) TestList_NonExistentDirectory() {
	store := OpenDir(filepath.Join(s.tempDir, "nonexistent"))
	list, err := store.List()
	require.NoError(s.T(), err)
	require.Nil(s.T(), list)
}

func (s *StoreSuite) TestList_SkipsSubdirectories() {
	subDir := filepath.Join(s.store.TicketsDir(), "subdir.md")
	require.NoError(s.T(), os.MkdirAll(subDir, 0755))

	ticket := &Ticket{
		ID:      "tic-real",
		Status:  StatusOpen,
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	list, err := s.store.List()
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 1)
	require.Equal(s.T(), "tic-real", list[0].ID)
}

func (s *StoreSuite) TestList_SkipsNonMdFiles() {
	txtFile := filepath.Join(s.store.TicketsDir(), "tic-fake.txt")
	require.NoError(s.T(), os.WriteFile(txtFile, []byte("not a ticket"), 0644))

	ticket := &Ticket{
		ID:      "tic-actual",
		Status:  StatusOpen,
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	list, err := s.store.List()
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 1)
	require.Equal(s.T(), "tic-actual", list[0].ID)
}

func (s *StoreSuite) TestStoreOperationsIgnoreReadme() {
	readmePath := filepath.Join(s.store.TicketsDir(), TicketReadmeName)
	require.NoError(s.T(), os.WriteFile(readmePath, []byte("# Project tickets\n"), 0644))

	list, err := s.store.List()
	require.NoError(s.T(), err)
	require.Empty(s.T(), list)

	ids, err := s.store.ListIDs()
	require.NoError(s.T(), err)
	require.Empty(s.T(), ids)

	_, err = s.store.ResolveID("README")
	require.ErrorContains(s.T(), err, "ticket not found")
}

func (s *StoreSuite) TestList_ReadError() {
	invalidFile := filepath.Join(s.store.TicketsDir(), "tic-invalid.md")
	require.NoError(s.T(), os.WriteFile(invalidFile, []byte("not valid yaml frontmatter"), 0644))

	list, err := s.store.List()
	require.Error(s.T(), err)
	require.Nil(s.T(), list)
}

func (s *StoreSuite) TestDelete() {
	ticket := &Ticket{
		ID:      "tic-del1",
		Status:  StatusOpen,
		Created: time.Now().UTC(),
	}

	require.NoError(s.T(), s.store.Write(ticket))
	require.True(s.T(), s.store.Exists("tic-del1"))

	require.NoError(s.T(), s.store.Delete("tic-del1"))
	require.False(s.T(), s.store.Exists("tic-del1"))
}

func (s *StoreSuite) TestExists() {
	require.False(s.T(), s.store.Exists("tic-nonexistent"))

	ticket := &Ticket{
		ID:      "tic-exists",
		Status:  StatusOpen,
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))
	require.True(s.T(), s.store.Exists("tic-exists"))
}

func (s *StoreSuite) TestResolveID() {
	tickets := []*Ticket{
		{ID: "tic-abc1", Status: StatusOpen, Created: time.Now().UTC()},
		{ID: "tic-def2", Status: StatusOpen, Created: time.Now().UTC()},
		{ID: "tic-abc3", Status: StatusOpen, Created: time.Now().UTC()},
	}

	for _, t := range tickets {
		require.NoError(s.T(), s.store.Write(t))
	}

	tests := []struct {
		name    string
		partial string
		want    string
		wantErr bool
	}{
		{name: "exact match", partial: "tic-abc1", want: "tic-abc1"},
		{name: "partial match unique", partial: "def2", want: "tic-def2"},
		{name: "partial match ambiguous", partial: "abc", wantErr: true},
		{name: "no match", partial: "xyz", wantErr: true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := s.store.ResolveID(tt.partial)
			if tt.wantErr {
				require.Error(s.T(), err)
				return
			}
			require.NoError(s.T(), err)
			require.Equal(s.T(), tt.want, got)
		})
	}
}

func (s *StoreSuite) TestListIDs() {
	tickets := []*Ticket{
		{ID: "tic-id1", Status: StatusOpen, Created: time.Now().UTC()},
		{ID: "tic-id2", Status: StatusOpen, Created: time.Now().UTC()},
	}

	for _, t := range tickets {
		require.NoError(s.T(), s.store.Write(t))
	}

	ids, err := s.store.ListIDs()
	require.NoError(s.T(), err)
	require.Len(s.T(), ids, 2)
	require.Contains(s.T(), ids, "tic-id1")
	require.Contains(s.T(), ids, "tic-id2")
}

func (s *StoreSuite) TestListIDs_EmptyDirectory() {
	ids, err := s.store.ListIDs()
	require.NoError(s.T(), err)
	require.Len(s.T(), ids, 0)
}

func (s *StoreSuite) TestListIDs_NonExistentDirectory() {
	store := OpenDir(filepath.Join(s.tempDir, "nonexistent-ids"))
	ids, err := store.ListIDs()
	require.NoError(s.T(), err)
	require.Nil(s.T(), ids)
}

func (s *StoreSuite) TestEnsureDir() {
	newDir := filepath.Join(s.tempDir, "new-tickets")
	store := OpenDir(newDir)

	require.NoError(s.T(), store.EnsureDir())

	info, err := os.Stat(newDir)
	require.NoError(s.T(), err)
	require.True(s.T(), info.IsDir())
}

func (s *StoreSuite) TestAtomicClaim_Success() {
	ticket := &Ticket{
		ID:      "tic-claim1",
		Status:  StatusOpen,
		Title:   "Test Claim",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	claimed, err := s.store.AtomicClaim("tic-claim1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), StatusInProgress, claimed.Status)

	// Verify file was updated
	read, err := s.store.Read("tic-claim1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), StatusInProgress, read.Status)
}

func (s *StoreSuite) TestAtomicClaimWithPersistsClaimMetadata() {
	ticket := &Ticket{
		ID:      "tic-claim-context",
		Status:  StatusOpen,
		Title:   "Claim with context",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	claimed, err := s.store.AtomicClaimWith("tic-claim-context", func(ticket *Ticket) error {
		ticket.BaseCommit = "abc123"
		ticket.Branch = "ticket/tic-claim-context"
		return nil
	})

	require.NoError(s.T(), err)
	require.Equal(s.T(), "abc123", claimed.BaseCommit)
	require.Equal(s.T(), "ticket/tic-claim-context", claimed.Branch)

	read, err := s.store.Read("tic-claim-context")
	require.NoError(s.T(), err)
	require.Equal(s.T(), claimed.BaseCommit, read.BaseCommit)
	require.Equal(s.T(), claimed.Branch, read.Branch)
}

func (s *StoreSuite) TestAtomicClaim_AlreadyInProgress() {
	ticket := &Ticket{
		ID:      "tic-claim2",
		Status:  StatusInProgress,
		Title:   "Already In Progress",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	_, err := s.store.AtomicClaim("tic-claim2")
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, ErrAlreadyClaimed)
}

func (s *StoreSuite) TestAtomicClaim_AlreadyClosed() {
	ticket := &Ticket{
		ID:      "tic-claim3",
		Status:  StatusClosed,
		Title:   "Already Closed",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	_, err := s.store.AtomicClaim("tic-claim3")
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, ErrAlreadyClaimed)
}

func (s *StoreSuite) TestAtomicClaim_ConcurrentClaims() {
	ticket := &Ticket{
		ID:      "tic-race1",
		Status:  StatusOpen,
		Title:   "Race Condition Test",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	const numWorkers = 10
	results := make(chan error, numWorkers)

	for range numWorkers {
		go func() {
			_, err := s.store.AtomicClaim("tic-race1")
			results <- err
		}()
	}

	var successCount, failCount int
	for range numWorkers {
		err := <-results
		if err == nil {
			successCount++
		} else {
			require.ErrorIs(s.T(), err, ErrAlreadyClaimed)
			failCount++
		}
	}

	require.Equal(s.T(), 1, successCount, "exactly one worker should claim successfully")
	require.Equal(s.T(), numWorkers-1, failCount, "all other workers should fail")

	read, err := s.store.Read("tic-race1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), StatusInProgress, read.Status)
}

func (s *StoreSuite) TestDelete_NonExistent() {
	err := s.store.Delete("nonexistent-ticket")
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "failed to delete ticket")
}

func (s *StoreSuite) TestSetExternalRef_Sets() {
	ticket := &Ticket{
		ID:      "tic-ext1",
		Status:  StatusOpen,
		Title:   "ExtRef Set",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetExternalRef("tic-ext1", "JIRA-456")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "JIRA-456", updated.ExternalRef)

	read, err := s.store.Read("tic-ext1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "JIRA-456", read.ExternalRef)
}

func (s *StoreSuite) TestSetExternalRef_Overwrites() {
	ticket := &Ticket{
		ID:          "tic-ext2",
		Status:      StatusOpen,
		ExternalRef: "gh-1",
		Title:       "ExtRef Overwrite",
		Created:     time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetExternalRef("tic-ext2", "gh-2")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "gh-2", updated.ExternalRef)
}

func (s *StoreSuite) TestSetExternalRef_ClearsWhenEmpty() {
	ticket := &Ticket{
		ID:          "tic-ext3",
		Status:      StatusOpen,
		ExternalRef: "gh-9",
		Title:       "ExtRef Clear",
		Created:     time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetExternalRef("tic-ext3", "")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "", updated.ExternalRef)

	read, err := s.store.Read("tic-ext3")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "", read.ExternalRef)
}

func (s *StoreSuite) TestSetExternalRef_NotFound() {
	_, err := s.store.SetExternalRef("nonexistent", "ref")
	require.Error(s.T(), err)
}

func (s *StoreSuite) TestSetPR_Sets() {
	ticket := &Ticket{
		ID:      "tic-pr1",
		Status:  StatusOpen,
		Title:   "PR Set",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetPR("tic-pr1", "gh-pr-42")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "gh-pr-42", updated.PR)

	read, err := s.store.Read("tic-pr1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "gh-pr-42", read.PR)
}

func (s *StoreSuite) TestSetPR_Overwrites() {
	ticket := &Ticket{
		ID:      "tic-pr2",
		Status:  StatusOpen,
		PR:      "!1",
		Title:   "PR Overwrite",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetPR("tic-pr2", "!2")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "!2", updated.PR)
}

func (s *StoreSuite) TestSetPR_ClearsWhenEmpty() {
	ticket := &Ticket{
		ID:      "tic-pr3",
		Status:  StatusOpen,
		PR:      "gh-pr-9",
		Title:   "PR Clear",
		Created: time.Now().UTC(),
	}
	require.NoError(s.T(), s.store.Write(ticket))

	updated, err := s.store.SetPR("tic-pr3", "")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "", updated.PR)

	read, err := s.store.Read("tic-pr3")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "", read.PR)
}

func (s *StoreSuite) TestSetPR_NotFound() {
	_, err := s.store.SetPR("nonexistent", "ref")
	require.Error(s.T(), err)
}

func (s *StoreSuite) TestAtomicClaim_FileNotFound() {
	_, err := s.store.AtomicClaim("nonexistent-ticket")
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "failed to open ticket file")
}

func (s *StoreSuite) TestRead_NotFound() {
	_, err := s.store.Read("nonexistent")
	require.Error(s.T(), err)
}

func (s *StoreSuite) TestFindTicketsDir() {
	nestedDir := filepath.Join(s.tempDir, "level1", "level2")
	require.NoError(s.T(), os.MkdirAll(nestedDir, 0755))

	originalDir, err := os.Getwd()
	require.NoError(s.T(), err)
	defer func() { _ = os.Chdir(originalDir) }()

	require.NoError(s.T(), os.Chdir(nestedDir))

	found, err := FindTicketsDir()
	require.NoError(s.T(), err)

	expected, err := filepath.EvalSymlinks(filepath.Join(s.tempDir, TicketsDirName))
	require.NoError(s.T(), err)
	actual, err := filepath.EvalSymlinks(found)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expected, actual)
}
