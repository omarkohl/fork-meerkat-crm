package domain

import (
	"fmt"
	"strings"
)

type GraphContact struct {
	ID             ContactID
	Firstname      string
	Lastname       string
	PhotoThumbnail string
	Circles        []string
}

type GraphLink struct {
	ID   RelationshipID
	From ContactID
	To   ContactID
	Type string
}

type GraphActivity struct {
	ID           uint
	Title        string
	Participants []ContactID
}

type Node struct {
	ID             string
	Type           string
	Label          string
	PhotoThumbnail string
	Circles        []string
}

type Edge struct {
	ID     string
	Source string
	Target string
	Type   string
	Label  string
}

type Graph struct {
	Nodes []Node
	Edges []Edge
}

// BuildGraph draws the contacts, the links between them and the activities that bring at least
// two of them together. The contacts given are the visible ones: an edge or an activity
// participant that is not among them is left out, so nothing points at a missing node.
func BuildGraph(contacts []GraphContact, links []GraphLink, activities []GraphActivity) Graph {
	g := Graph{Nodes: make([]Node, 0, len(contacts)+len(activities)), Edges: []Edge{}}
	visible := make(map[ContactID]bool, len(contacts))
	for _, c := range contacts {
		visible[c.ID] = true
		label := strings.TrimSpace(c.Firstname + " " + c.Lastname)
		if label == "" {
			label = "Unknown"
		}
		g.Nodes = append(g.Nodes, Node{
			ID: fmt.Sprintf("c-%d", c.ID), Type: "contact", Label: label, PhotoThumbnail: c.PhotoThumbnail, Circles: c.Circles,
		})
	}

	participants := make(map[uint][]ContactID)
	for _, a := range activities {
		for _, p := range a.Participants {
			if visible[p] {
				participants[a.ID] = append(participants[a.ID], p)
			}
		}
		if len(participants[a.ID]) >= 2 {
			g.Nodes = append(g.Nodes, Node{ID: fmt.Sprintf("a-%d", a.ID), Type: "activity", Label: a.Title})
		}
	}

	for _, l := range links {
		if visible[l.From] && visible[l.To] {
			g.Edges = append(g.Edges, Edge{
				ID: fmt.Sprintf("r-%d", l.ID), Source: fmt.Sprintf("c-%d", l.From), Target: fmt.Sprintf("c-%d", l.To),
				Type: "relationship", Label: l.Type,
			})
		}
	}
	for _, a := range activities {
		if len(participants[a.ID]) < 2 {
			continue
		}
		for _, p := range participants[a.ID] {
			g.Edges = append(g.Edges, Edge{
				ID: fmt.Sprintf("ae-%d-%d", a.ID, p), Source: fmt.Sprintf("a-%d", a.ID), Target: fmt.Sprintf("c-%d", p),
				Type: "activity", Label: a.Title,
			})
		}
	}
	return g
}
