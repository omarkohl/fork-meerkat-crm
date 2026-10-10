package controllers

import (
	"fmt"
	"meerkat/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetGraph returns the network graph data for visualization
func GetGraph(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	// 1. Fetch all contacts (minimal fields for performance), excluding archived
	var contacts []models.Contact
	if err := db.Select("id", "firstname", "lastname", "photo_thumbnail", "circles").
		Where("user_id = ? AND archived = ?", userID, false).
		Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contacts"})
		return
	}

	// 2. Fetch all relationships where related_contact_id is set (linked relationships)
	var relationships []models.Relationship
	if err := db.Where("user_id = ? AND related_contact_id IS NOT NULL", userID).
		Find(&relationships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch relationships"})
		return
	}

	// 3. Fetch all activities with their contacts
	var activities []models.Activity
	if err := db.Preload("Contacts", func(db *gorm.DB) *gorm.DB {
		return db.Select("id").Where("user_id = ?", userID)
	}).Where("user_id = ?", userID).Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activities"})
		return
	}

	// Build nodes array
	nodes := make([]models.GraphNode, 0, len(contacts)+len(activities))

	// Add contact nodes
	for _, contact := range contacts {
		label := strings.TrimSpace(contact.Firstname + " " + contact.Lastname)
		if label == "" {
			label = "Unknown"
		}
		nodes = append(nodes, models.GraphNode{
			ID:             fmt.Sprintf("c-%d", contact.ID),
			Type:           "contact",
			Label:          label,
			PhotoThumbnail: contact.PhotoThumbnail,
			Circles:        contact.Circles,
		})
	}

	// Archived contacts are not nodes, so nothing may point at them: edges are only
	// drawn between nodes that exist.
	visibleContacts := make(map[uint]bool, len(contacts))
	for _, contact := range contacts {
		visibleContacts[contact.ID] = true
	}
	participants := func(a models.Activity) []models.Contact {
		visible := make([]models.Contact, 0, len(a.Contacts))
		for _, contact := range a.Contacts {
			if visibleContacts[contact.ID] {
				visible = append(visible, contact)
			}
		}
		return visible
	}

	// Add activity nodes (only for activities with 2+ visible contacts)
	activityNodeIDs := make(map[uint]bool)
	for _, activity := range activities {
		if len(participants(activity)) >= 2 {
			nodes = append(nodes, models.GraphNode{
				ID:    fmt.Sprintf("a-%d", activity.ID),
				Type:  "activity",
				Label: activity.Title,
			})
			activityNodeIDs[activity.ID] = true
		}
	}

	// Build edges array
	edges := make([]models.GraphEdge, 0)

	// Add relationship edges (contact -> contact)
	for _, rel := range relationships {
		if rel.RelatedContactID != nil && visibleContacts[rel.ContactID] && visibleContacts[*rel.RelatedContactID] {
			edges = append(edges, models.GraphEdge{
				ID:     fmt.Sprintf("r-%d", rel.ID),
				Source: fmt.Sprintf("c-%d", rel.ContactID),
				Target: fmt.Sprintf("c-%d", *rel.RelatedContactID),
				Type:   "relationship",
				Label:  rel.Type,
			})
		}
	}

	// Add activity edges (star pattern: activity node -> each participating contact)
	for _, activity := range activities {
		if activityNodeIDs[activity.ID] {
			activityNodeID := fmt.Sprintf("a-%d", activity.ID)
			for _, contact := range participants(activity) {
				edges = append(edges, models.GraphEdge{
					ID:     fmt.Sprintf("ae-%d-%d", activity.ID, contact.ID),
					Source: activityNodeID,
					Target: fmt.Sprintf("c-%d", contact.ID),
					Type:   "activity",
					Label:  activity.Title,
				})
			}
		}
	}

	c.JSON(http.StatusOK, models.GraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}
