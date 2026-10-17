package domain_test

import (
	"testing"

	"meerkat/relationships/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestEditReplacesWhatTheUserSetsButNotTheContact(t *testing.T) {
	was := domain.ContactID(5)
	r := domain.Relationship{ContactID: 1, Name: "old", RelatedContactID: &was}

	r.Edit(domain.Relationship{ContactID: 9, Name: "new", Type: "Sibling", Gender: "male", Birthday: "1990-01-02"})

	assert.Equal(t, domain.ContactID(1), r.ContactID)
	assert.Equal(t, "new", r.Name)
	assert.Equal(t, "Sibling", r.Type)
	assert.Nil(t, r.RelatedContactID)
}

func TestGraphLeavesOutEverythingThatPointsAtAMissingNode(t *testing.T) {
	contacts := []domain.GraphContact{{ID: 1, Firstname: "Ann"}, {ID: 2, Firstname: "Bob", Lastname: "B"}, {ID: 3}}
	links := []domain.GraphLink{{ID: 1, From: 1, To: 2, Type: "Friend"}, {ID: 2, From: 1, To: 99, Type: "Friend"}}
	activities := []domain.GraphActivity{
		{ID: 1, Title: "Dinner", Participants: []domain.ContactID{1, 2, 99}},
		{ID: 2, Title: "Hike", Participants: []domain.ContactID{1, 99}},
	}

	g := domain.BuildGraph(contacts, links, activities)

	var nodes, edges []string
	for _, n := range g.Nodes {
		nodes = append(nodes, n.ID)
	}
	for _, e := range g.Edges {
		edges = append(edges, e.ID)
	}
	assert.Equal(t, []string{"c-1", "c-2", "c-3", "a-1"}, nodes)
	assert.Equal(t, []string{"r-1", "ae-1-1", "ae-1-2"}, edges)
	assert.Equal(t, "Unknown", g.Nodes[2].Label)
	assert.Equal(t, "Bob B", g.Nodes[1].Label)
}

func TestAnEmptyGraphHasEmptyListsNotNil(t *testing.T) {
	g := domain.BuildGraph(nil, nil, nil)
	assert.NotNil(t, g.Nodes)
	assert.NotNil(t, g.Edges)
}
