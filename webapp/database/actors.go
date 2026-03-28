package database

import "gorm.io/gorm"

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

type RegulatedEntitySite struct {
	gorm.Model
	RegulatedEntityID uint               `gorm:"not null;index"`
	RegulatedEntity   *RegulatedEntities `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SiteAddress       string
	SiteContactPerson string
}

type EnvironmentalOfficer struct {
	gorm.Model
	Name string
}

type OPS struct {
	gorm.Model
	Name string
}
