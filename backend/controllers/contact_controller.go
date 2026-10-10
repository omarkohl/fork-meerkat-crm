package controllers

import (
	"errors"
	apperrors "meerkat/errors"
	"meerkat/logger"
	"meerkat/middleware"
	"meerkat/models"
	"meerkat/search"
	"meerkat/services"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateContact(c *gin.Context) {
	// Save to the database
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	// Get validated input from validation middleware
	contactInput, err := middleware.GetValidated[models.ContactInput](c)
	if err != nil {
		apperrors.AbortWithError(c, err)
		return
	}

	// Create contact from validated input
	contact := models.Contact{
		UserID:             userID,
		Firstname:          contactInput.Firstname,
		Lastname:           contactInput.Lastname,
		Nickname:           contactInput.Nickname,
		Gender:             contactInput.Gender,
		Email:              contactInput.Email,
		Phone:              contactInput.Phone,
		Birthday:           contactInput.Birthday,
		Address:            contactInput.Address,
		HowWeMet:           contactInput.HowWeMet,
		FoodPreference:     contactInput.FoodPreference,
		WorkInformation:    contactInput.WorkInformation,
		ContactInformation: contactInput.ContactInformation,
		Circles:            contactInput.Circles,
		CustomFields:       contactInput.CustomFields,
		Emails:             contactInput.Emails,
		Phones:             contactInput.Phones,
		Addresses:          contactInput.Addresses,
		URLs:               contactInput.URLs,
		IMPPs:              contactInput.IMPPs,
		Prefix:             contactInput.Prefix,
		MiddleName:         contactInput.MiddleName,
		Suffix:             contactInput.Suffix,
		Organization:       contactInput.Organization,
		Department:         contactInput.Department,
		JobTitle:           contactInput.JobTitle,
		Role:               contactInput.Role,
		Anniversary:        contactInput.Anniversary,
	}
	if err := db.Create(&contact).Error; err != nil {
		logger.FromContext(c).Error().Err(err).Msg("Error saving contact to database")
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to save contact").WithError(err))
		return
	}

	go services.TriggerWebhooks(db, currentConfig(c), userID, "contact.created", contact)
	c.JSON(http.StatusCreated, gin.H{"message": "Contact created successfully", "contact": contact})
}

// contactFTSMatch returns the FTS5 expression for a search term, or "" when the LIKE search
// has to be used (no index, or nothing but punctuation was typed).
func contactFTSMatch(db *gorm.DB, searchTerm string) string {
	match, ok := search.MatchQuery(searchTerm)
	if !ok || !search.Available(db, "contacts_fts") {
		return ""
	}
	return match
}

// filters a contacts query by a free-text term: the FTS index (accent- and case-insensitive,
// every word a prefix) when there is one, otherwise a LIKE search
func applyContactSearch(query *gorm.DB, searchTerm, ftsMatch string) *gorm.DB {
	if ftsMatch != "" {
		return query.Joins("JOIN "+search.Hits("contacts_fts")+" AS hits ON hits.hit_id = contacts.id", ftsMatch)
	}
	like := "%" + searchTerm + "%"
	return query.Where(
		"firstname LIKE ? OR lastname LIKE ? OR nickname LIKE ? "+
			"OR (firstname || ' ' || lastname) LIKE ? OR (nickname || ' ' || lastname) LIKE ? "+
			"OR email LIKE ? OR phone LIKE ? "+
			"OR (json_valid(emails) AND EXISTS (SELECT 1 FROM json_each(contacts.emails) WHERE json_extract(json_each.value, '$.value') LIKE ?)) "+
			"OR (json_valid(phones) AND EXISTS (SELECT 1 FROM json_each(contacts.phones) WHERE json_extract(json_each.value, '$.value') LIKE ?))",
		like, like, like, like, like, like, like, like, like,
	)
}

func GetContacts(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	pagination := GetPaginationParams(c)

	// Define allowed fields and parse requested fields with validation
	allowedFields := []string{"ID", "firstname", "lastname", "nickname", "gender", "email", "phone", "birthday", "address", "how_we_met", "food_preference", "work_information", "contact_information", "circles", "photo", "photo_thumbnail", "custom_fields", "archived", "emails", "phones", "addresses", "urls", "impps", "prefix", "middle_name", "suffix", "organization", "department", "job_title", "role", "anniversary"}
	var selectedFields []string
	fields := c.Query("fields")
	if fields != "" {
		for _, field := range strings.Split(fields, ",") {
			if slices.Contains(allowedFields, field) { // Validate field
				selectedFields = append(selectedFields, field)
			}
		}
	} else {
		selectedFields = allowedFields // Use all allowed fields if none are specified
	}

	// Parse relationships to include with validation
	var relationshipMap = map[string]bool{
		"notes":         false,
		"activities":    false,
		"relationships": false,
		"reminders":     false,
	}
	includes := c.Query("includes")
	for _, rel := range strings.Split(includes, ",") {
		if _, exists := relationshipMap[rel]; exists {
			relationshipMap[rel] = true
		}
	}

	// Parse and validate sort parameters
	sortField := c.DefaultQuery("sort", "id")
	sortOrder := c.DefaultQuery("order", "desc")

	allowedSortFields := map[string]bool{"firstname": true, "lastname": true, "id": true, "random": true}
	if !allowedSortFields[sortField] {
		sortField = "id"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	// Parse archive filtering parameters
	includeArchived := c.Query("include_archived") == "true"
	archivedOnly := c.Query("archived") == "true"

	var contacts []models.Contact
	query := db.Model(&models.Contact{}).Where("user_id = ?", userID).Limit(pagination.Limit).Offset(pagination.Offset)

	// Apply archive filtering
	if !includeArchived {
		if archivedOnly {
			query = query.Where("archived = ?", true)
		} else {
			query = query.Where("archived = ?", false)
		}
	}

	// Apply ordering - random uses RANDOM() function, others use column name
	// For search with include_archived, order non-archived first
	if includeArchived && c.Query("search") != "" {
		query = query.Order("archived ASC")
	}
	searchTerm := c.Query("search")
	ftsMatch := ""
	if searchTerm != "" {
		ftsMatch = contactFTSMatch(db, searchTerm)
	}
	// Best match first, unless the caller asked for an order
	if ftsMatch != "" && c.Query("sort") == "" {
		query = query.Order("hits.hit_rank").Order("contacts.id DESC")
	} else if sortField == "random" {
		query = query.Order("RANDOM()")
	} else {
		query = query.Order(sortField + " " + sortOrder)
	}

	if len(selectedFields) > 0 {
		query = query.Select(selectedFields)
	}

	// Apply search filter using parameterization
	if searchTerm != "" {
		query = applyContactSearch(query, searchTerm, ftsMatch)
	}

	if circle := c.Query("circle"); circle != "" {
		query = query.Where("EXISTS (SELECT 1 FROM json_each(contacts.circles) WHERE json_each.value = ?)", circle)
	}

	// Preload requested relationships
	for rel, include := range relationshipMap {
		if include {
			switch rel {
			case "notes":
				query = query.Preload("Notes", "notes.user_id = ?", userID)
			case "activities":
				query = query.Preload("Activities", "activities.user_id = ?", userID)
			case "relationships":
				query = query.Preload("Relationships", "relationships.user_id = ?", userID)
			case "reminders":
				query = query.Preload("Reminders", "reminders.user_id = ?", userID)
			}
		}
	}

	// Execute query
	if err := query.Find(&contacts).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contacts").WithError(err))
		return
	}

	var total int64
	countQuery := db.Model(&models.Contact{}).Where("user_id = ?", userID)

	// Apply the same archive filter to the count query
	if !includeArchived {
		if archivedOnly {
			countQuery = countQuery.Where("archived = ?", true)
		} else {
			countQuery = countQuery.Where("archived = ?", false)
		}
	}

	// Apply the same search filters to the count query
	if searchTerm != "" {
		countQuery = applyContactSearch(countQuery, searchTerm, ftsMatch)
	}

	if circle := c.Query("circle"); circle != "" {
		countQuery = countQuery.Where("EXISTS (SELECT 1 FROM json_each(contacts.circles) WHERE json_each.value = ?)", circle)
	}

	countQuery.Count(&total)

	// Map contacts to ContactResponse with photo thumbnails
	contactResponses := make([]models.ContactResponse, len(contacts))
	for i, contact := range contacts {
		contactResponses[i] = models.ContactResponse{
			Contact:        contact,
			PhotoThumbnail: contact.PhotoThumbnail,
		}
	}

	// Respond with contacts and pagination metadata
	c.JSON(http.StatusOK, gin.H{
		"contacts": contactResponses,
		"total":    total,
		"page":     pagination.Page,
		"limit":    pagination.Limit,
	})
}

func GetContactsRandom(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var selectedFields = []string{"ID", "firstname", "lastname", "nickname", "circles", "photo_thumbnail"}

	var contacts []models.Contact
	query := db.Model(&models.Contact{}).Where("user_id = ?", userID).Where("archived = ?", false)

	if len(selectedFields) > 0 {
		query = query.Select(selectedFields)
	}

	// Get 5 random contacts
	query = query.Order("RANDOM()").Limit(5)

	// Execute query
	if err := query.Find(&contacts).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contacts").WithError(err))
		return
	}

	// Map to response with photo thumbnail
	contactResponses := make([]models.ContactResponse, len(contacts))
	for i, contact := range contacts {
		contactResponses[i] = models.ContactResponse{
			Contact:        contact,
			PhotoThumbnail: contact.PhotoThumbnail,
		}
	}

	// Respond with random contacts
	c.JSON(http.StatusOK, gin.H{
		"contacts": contactResponses,
	})
}

func GetUpcomingBirthdays(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	birthdays, err := services.GetUpcomingBirthdays(db, userID, time.Now())
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve upcoming birthdays").WithError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"birthdays": birthdays,
	})
}

func GetContact(c *gin.Context) {
	id := c.Param("id")

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	db := c.MustGet("db").(*gorm.DB)

	// Check for fields query parameter to enable partial fetching
	allowedFields := []string{"ID", "firstname", "lastname", "nickname", "gender", "email", "phone", "birthday", "address", "how_we_met", "food_preference", "work_information", "contact_information", "circles", "photo", "photo_thumbnail", "custom_fields", "archived", "emails", "phones", "addresses", "urls", "impps", "prefix", "middle_name", "suffix", "organization", "department", "job_title", "role", "anniversary"}
	var selectedFields []string
	fields := c.Query("fields")
	if fields != "" {
		for _, field := range strings.Split(fields, ",") {
			if slices.Contains(allowedFields, field) {
				selectedFields = append(selectedFields, field)
			}
		}
	}

	var contact models.Contact
	query := db.Where("user_id = ?", userID)

	if len(selectedFields) > 0 {
		// Partial fetch: only select requested fields, skip preloading associations
		query = query.Select(selectedFields)
	} else {
		// Full fetch: preload all associations
		query = query.
			Preload("Notes", "notes.user_id = ?", userID).
			Preload("Activities", "activities.user_id = ?", userID).
			Preload("Relationships", "relationships.user_id = ?", userID).
			Preload("Reminders", "reminders.user_id = ?", userID)
	}

	if err := query.First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", id))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contact").WithError(err))
		}
		return
	}
	c.JSON(http.StatusOK, contact)
}

func UpdateContact(c *gin.Context) {
	id := c.Param("id")
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var contact models.Contact
	if err := db.Where("user_id = ?", userID).First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", id))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contact").WithError(err))
		}
		return
	}

	// Get validated input from validation middleware
	contactInput, err := middleware.GetValidated[models.ContactInput](c)
	if err != nil {
		apperrors.AbortWithError(c, err)
		return
	}

	// Updateable fields
	contact.Firstname = contactInput.Firstname
	contact.Lastname = contactInput.Lastname
	contact.Nickname = contactInput.Nickname
	contact.Gender = contactInput.Gender
	contact.Email = contactInput.Email
	contact.Phone = contactInput.Phone
	contact.Birthday = contactInput.Birthday
	contact.Address = contactInput.Address
	contact.HowWeMet = contactInput.HowWeMet
	contact.FoodPreference = contactInput.FoodPreference
	contact.WorkInformation = contactInput.WorkInformation
	contact.ContactInformation = contactInput.ContactInformation
	contact.Circles = contactInput.Circles
	contact.CustomFields = contactInput.CustomFields
	contact.Emails = contactInput.Emails
	contact.Phones = contactInput.Phones
	contact.Addresses = contactInput.Addresses
	contact.URLs = contactInput.URLs
	contact.IMPPs = contactInput.IMPPs
	contact.Prefix = contactInput.Prefix
	contact.MiddleName = contactInput.MiddleName
	contact.Suffix = contactInput.Suffix
	contact.Organization = contactInput.Organization
	contact.Department = contactInput.Department
	contact.JobTitle = contactInput.JobTitle
	contact.Role = contactInput.Role
	contact.Anniversary = contactInput.Anniversary

	if err := db.Save(&contact).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to update contact").WithError(err))
		return
	}

	go services.TriggerWebhooks(db, currentConfig(c), userID, "contact.updated", contact)
	c.JSON(http.StatusOK, contact)
}

func DeleteContact(c *gin.Context) {
	id := c.Param("id")
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	// Check if contact exists first
	var contact models.Contact
	if err := db.Where("user_id = ?", userID).First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", id))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contact").WithError(err))
		}
		return
	}

	// Soft-delete the contact and its notes, reminders and relationships together.
	// Activity links and photo files stay so that restoring from the trash is lossless;
	// the purge (services.PurgeTrashedContacts) removes them for good.
	err := db.Transaction(func(tx *gorm.DB) error {
		return models.TrashContact(tx, userID, contact.ID)
	})

	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to delete contact and associated data").WithError(err))
		return
	}

	go services.TriggerWebhooks(db, currentConfig(c), userID, "contact.deleted", gin.H{"id": contact.ID})
	c.JSON(http.StatusOK, gin.H{"message": "Contact deleted"})
}

// GetCircles returns all unique circles associated with contacts.
func GetCircles(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var circleNames []string

	// Raw SQL query to extract unique circle names
	err := db.Raw(`SELECT DISTINCT json_each.value AS circle
	               FROM contacts, json_each(contacts.circles)
	               WHERE contacts.user_id = ? AND contacts.deleted_at IS NULL`, userID).Scan(&circleNames).Error
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve circles").WithError(err))
		return
	}

	// Return the list of unique circle names
	c.JSON(http.StatusOK, circleNames)
}

// ArchiveContact archives a contact and deletes all its reminders
func ArchiveContact(c *gin.Context) {
	id := c.Param("id")
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var contact models.Contact
	if err := db.Where("user_id = ?", userID).First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", id))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contact").WithError(err))
		}
		return
	}

	// Archive contact and delete reminders in a transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		// Delete all reminders for this contact
		if err := tx.Where("contact_id = ? AND user_id = ?", id, userID).Delete(&models.Reminder{}).Error; err != nil {
			return err
		}

		// Set archived to true
		if err := tx.Model(&contact).Update("archived", true).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to archive contact").WithError(err))
		return
	}

	contact.Archived = true
	c.JSON(http.StatusOK, contact)
}

// UnarchiveContact restores an archived contact
func UnarchiveContact(c *gin.Context) {
	id := c.Param("id")
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var contact models.Contact
	if err := db.Where("user_id = ?", userID).First(&contact, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact").WithDetails("id", id))
		} else {
			apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to retrieve contact").WithError(err))
		}
		return
	}

	if err := db.Model(&contact).Update("archived", false).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("Failed to unarchive contact").WithError(err))
		return
	}

	contact.Archived = false
	c.JSON(http.StatusOK, contact)
}
