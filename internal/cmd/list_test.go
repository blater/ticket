package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tk "github.com/radutopala/ticket/pkg/ticket"
)

func (s *CmdSuite) TestListStatusDefaults() {
	s.createTestTicket("tic-open", tk.StatusOpen, "Open ticket")
	s.createTestTicket("tic-active", tk.StatusInProgress, "Active ticket")
	s.createTestTicket("tic-closed", tk.StatusClosed, "Closed ticket")
	for _, id := range []string{"tic-open", "tic-closed"} {
		ticket, err := store.Read(id)
		require.NoError(s.T(), err)
		ticket.Assignee = "alice"
		ticket.Tags = []string{"backend"}
		ticket.Type = tk.TypeBug
		require.NoError(s.T(), store.Write(ticket))
	}

	originalFilter, originalSort, originalAll := listFlags, sortFlags, listAll
	s.T().Cleanup(func() {
		listFlags, sortFlags, listAll = originalFilter, originalSort, originalAll
	})
	for _, command := range []string{"list", "ls"} {
		for _, tc := range []struct {
			name string
			args []string
			want []string
		}{
			{"default", nil, []string{"tic-active", "tic-open"}},
			{"all", []string{"--all"}, []string{"tic-active", "tic-closed", "tic-open"}},
			{"open", []string{"--status", "open"}, []string{"tic-open"}},
			{"in_progress", []string{"--status", "in_progress"}, []string{"tic-active"}},
			{"closed", []string{"--status", "closed"}, []string{"tic-closed"}},
			{"all with status", []string{"--all", "--status", "closed"}, []string{"tic-closed"}},
			{"assignee", []string{"--assignee", "alice"}, []string{"tic-open"}},
			{"tag", []string{"--tag", "backend"}, []string{"tic-open"}},
			{"type", []string{"--type", "bug"}, []string{"tic-open"}},
			{"all with filter", []string{"--all", "--assignee", "alice"}, []string{"tic-closed", "tic-open"}},
			{"no matches", []string{"--assignee", "nobody"}, nil},
		} {
			s.Run(command+"/"+tc.name, func() {
				listFlags, sortFlags, listAll = tk.FilterOptions{}, tk.SortOptions{}, false
				args := append([]string{command, "--json"}, tc.args...)
				output, err := s.executeCommand(args...)
				require.NoError(s.T(), err)
				var tickets []*tk.Ticket
				require.NoError(s.T(), json.Unmarshal([]byte(output), &tickets))
				var ids []string
				for _, ticket := range tickets {
					ids = append(ids, ticket.ID)
				}
				require.Equal(s.T(), tc.want, ids)
			})
		}
	}
}

type ListSuite struct {
	suite.Suite
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) TestHasTag() {
	tests := []struct {
		name     string
		tags     []string
		tag      string
		expected bool
	}{
		{
			name:     "tag exists exact match",
			tags:     []string{"backend", "api", "urgent"},
			tag:      "api",
			expected: true,
		},
		{
			name:     "tag exists case insensitive",
			tags:     []string{"Backend", "API", "Urgent"},
			tag:      "api",
			expected: true,
		},
		{
			name:     "tag not found",
			tags:     []string{"backend", "api", "urgent"},
			tag:      "frontend",
			expected: false,
		},
		{
			name:     "empty tags",
			tags:     []string{},
			tag:      "api",
			expected: false,
		},
		{
			name:     "nil tags",
			tags:     nil,
			tag:      "api",
			expected: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := tk.HasTag(tt.tags, tt.tag)
			require.Equal(s.T(), tt.expected, result)
		})
	}
}

func (s *ListSuite) TestFilterTickets() {
	now := time.Now()
	tickets := []*tk.Ticket{
		{ID: "t1", Status: tk.StatusOpen, Assignee: "alice", Tags: []string{"backend"}, Created: now},
		{ID: "t2", Status: tk.StatusInProgress, Assignee: "bob", Tags: []string{"frontend"}, Created: now},
		{ID: "t3", Status: tk.StatusClosed, Assignee: "alice", Tags: []string{"backend", "urgent"}, Created: now},
		{ID: "t4", Status: tk.StatusOpen, Assignee: "charlie", Tags: []string{"api"}, Created: now},
	}

	tests := []struct {
		name     string
		status   string
		assignee string
		tag      string
		wantIDs  []string
	}{
		{
			name:    "no filters",
			wantIDs: []string{"t1", "t2", "t3", "t4"},
		},
		{
			name:    "filter by status open",
			status:  "open",
			wantIDs: []string{"t1", "t4"},
		},
		{
			name:    "filter by status in_progress",
			status:  "in_progress",
			wantIDs: []string{"t2"},
		},
		{
			name:     "filter by assignee",
			assignee: "alice",
			wantIDs:  []string{"t1", "t3"},
		},
		{
			name:    "filter by tag",
			tag:     "backend",
			wantIDs: []string{"t1", "t3"},
		},
		{
			name:     "filter by status and assignee",
			status:   "open",
			assignee: "alice",
			wantIDs:  []string{"t1"},
		},
		{
			name:     "filter by assignee and tag",
			assignee: "alice",
			tag:      "urgent",
			wantIDs:  []string{"t3"},
		},
		{
			name:    "no matches",
			status:  "open",
			tag:     "nonexistent",
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			opts := tk.FilterOptions{
				Status:   tt.status,
				Assignee: tt.assignee,
				Tag:      tt.tag,
			}
			result := tk.Filter(tickets, opts)

			var ids []string
			for _, t := range result {
				ids = append(ids, t.ID)
			}

			require.Equal(s.T(), tt.wantIDs, ids)
		})
	}
}

func (s *ListSuite) TestSortTicketsDefaultPriority() {
	tests := []struct {
		name    string
		tickets []*tk.Ticket
		wantIDs []string
	}{
		{
			name: "sort by priority ascending",
			tickets: []*tk.Ticket{
				{ID: "t3", Priority: 3},
				{ID: "t1", Priority: 1},
				{ID: "t2", Priority: 2},
			},
			wantIDs: []string{"t1", "t2", "t3"},
		},
		{
			name: "same priority sort by ID",
			tickets: []*tk.Ticket{
				{ID: "c", Priority: 1},
				{ID: "a", Priority: 1},
				{ID: "b", Priority: 1},
			},
			wantIDs: []string{"a", "b", "c"},
		},
		{
			name: "mixed priority and ID",
			tickets: []*tk.Ticket{
				{ID: "t2", Priority: 2},
				{ID: "t3", Priority: 1},
				{ID: "t1", Priority: 2},
				{ID: "t4", Priority: 0},
			},
			wantIDs: []string{"t4", "t3", "t1", "t2"},
		},
		{
			name:    "empty list",
			tickets: []*tk.Ticket{},
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tickets := make([]*tk.Ticket, len(tt.tickets))
			copy(tickets, tt.tickets)

			tk.Sort(tickets, tk.SortOptions{})

			var ids []string
			for _, t := range tickets {
				ids = append(ids, t.ID)
			}

			require.Equal(s.T(), tt.wantIDs, ids)
		})
	}
}

func (s *ListSuite) TestSortTickets() {
	now := time.Now()
	tickets := []*tk.Ticket{
		{ID: "t1", Priority: 2, Status: tk.StatusOpen, Title: "Beta feature", Created: now.Add(-3 * time.Hour)},
		{ID: "t2", Priority: 1, Status: tk.StatusClosed, Title: "Alpha bug", Created: now.Add(-1 * time.Hour)},
		{ID: "t3", Priority: 3, Status: tk.StatusInProgress, Title: "Gamma task", Created: now.Add(-2 * time.Hour)},
	}

	tests := []struct {
		name    string
		sortBy  string
		reverse bool
		wantIDs []string
	}{
		{
			name:    "sort by priority default",
			sortBy:  "",
			wantIDs: []string{"t2", "t1", "t3"},
		},
		{
			name:    "sort by priority explicit",
			sortBy:  "priority",
			wantIDs: []string{"t2", "t1", "t3"},
		},
		{
			name:    "sort by priority reversed",
			sortBy:  "priority",
			reverse: true,
			wantIDs: []string{"t3", "t1", "t2"},
		},
		{
			name:    "sort by created",
			sortBy:  "created",
			wantIDs: []string{"t1", "t3", "t2"},
		},
		{
			name:    "sort by created reversed",
			sortBy:  "created",
			reverse: true,
			wantIDs: []string{"t2", "t3", "t1"},
		},
		{
			name:    "sort by status",
			sortBy:  "status",
			wantIDs: []string{"t2", "t3", "t1"},
		},
		{
			name:    "sort by title",
			sortBy:  "title",
			wantIDs: []string{"t2", "t1", "t3"},
		},
		{
			name:    "sort by title reversed",
			sortBy:  "title",
			reverse: true,
			wantIDs: []string{"t3", "t1", "t2"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			ticketsCopy := make([]*tk.Ticket, len(tickets))
			copy(ticketsCopy, tickets)

			tk.Sort(ticketsCopy, tk.SortOptions{SortBy: tt.sortBy, Reverse: tt.reverse})

			var ids []string
			for _, t := range ticketsCopy {
				ids = append(ids, t.ID)
			}

			require.Equal(s.T(), tt.wantIDs, ids)
		})
	}
}
