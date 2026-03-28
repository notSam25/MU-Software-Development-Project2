package database

import (
	"time"

	"gorm.io/gorm"
)

type PermitRequest struct {
	gorm.Model
	RegulatedEntityID     uint
	RegulatedEntity       *RegulatedEntities    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	EnvironmentalPermitID uint                  `gorm:"not null;index"`
	EnvironmentalPermit   *EnvironmentalPermits `gorm:"foreignKey:EnvironmentalPermitID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	RequestNumber         string                `gorm:"uniqueIndex;not null"`
	ActivityDescription   string
	ActivityStartDate     time.Time
	ActivityDuration      time.Duration
	PermitFee             float64
	Decision              *PermitRequestDecision `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Payment               *Payment               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Permit                *Permit                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type PermitRequestDecision struct {
	gorm.Model
	PermitRequestID uint           `gorm:"not null;uniqueIndex"`
	Request         *PermitRequest `gorm:"foreignKey:PermitRequestID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	FinalDecision   string
	Description     string
}

type EnvironmentalPermits struct {
	gorm.Model
	PermitName  string `gorm:"not null"`
	PermitFee   float64
	Description string
}

type Payment struct {
	gorm.Model
	PermitRequestID      uint           `gorm:"not null;uniqueIndex"`
	Request              *PermitRequest `gorm:"foreignKey:PermitRequestID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	PaymentMethod        string
	LastFourDigitsOfCard string
	CardHolderName       string
	PaymentApproved      bool
}

type Permit struct {
	gorm.Model
	PermitRequestID uint           `gorm:"not null;uniqueIndex"`
	Request         *PermitRequest `gorm:"foreignKey:PermitRequestID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
