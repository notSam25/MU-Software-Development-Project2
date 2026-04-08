package database

import "gorm.io/gorm"

// RegulatedEntities represents organizations that need environmental permits
// They can register accounts, request permits, and submit payments
type RegulatedEntities struct {
	gorm.Model
	ContactPersonName   string                `json:"contact_person_name"`
	Password            string                `json:"password"`
	Email               string                `gorm:"uniqueIndex;not null" json:"email"`
	OrganizationName    string                `json:"organization_name"`
	OrganizationAddress string                `json:"organization_address"`
	Sites               []RegulatedEntitySite `gorm:"foreignKey:RegulatedEntityID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	PermitRequests      []PermitRequest       `gorm:"foreignKey:RegulatedEntityID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// RegulatedEntitySite represents physical locations associated with a regulated entity
// Used for tracking multiple sites that may require permits
type RegulatedEntitySite struct {
	gorm.Model
	RegulatedEntityID uint               `gorm:"not null;index"`
	RegulatedEntity   *RegulatedEntities `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SiteAddress       string
	SiteContactPerson string
}

// EnvironmentalOfficer represents government environmental officers
// They review permit requests and make final accept/reject decisions
type EnvironmentalOfficer struct {
	gorm.Model
	Name     string `json:"name"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `json:"password"`
}

// OPS represents Operations personnel who were previously used to review payment submissions.
// Payments are now auto-approved on submission; this actor is retained for schema compatibility.
type OPS struct {
	gorm.Model
	Name     string
	Email    string `gorm:"uniqueIndex;not null"`
	Password string
}
