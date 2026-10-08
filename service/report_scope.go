package service

import (
	"errors"

	"igb-leads-go/model"
)

// ReportScope filters the overview report to the campaigns the caller may see.
type ReportScope struct {
	CampaignID       *string
	RegionID         *string
	TrafficManagerID *string
	OwnerID          *string
}

var (
	ErrRegionRequired  = errors.New("usuário não possui região atribuída")
	ErrRegionForbidden = errors.New("região fora do seu escopo")
)

// ResolveReportScope maps role + query filters to campaign filters.
// Admin sees everything; supervisor is locked to their region; executive to
// their assigned campaigns; other roles to campaigns they own.
func ResolveReportScope(role, userID string, userRegionID *string, qCampaignID, qRegionID string) (ReportScope, error) {
	var scope ReportScope
	if qCampaignID != "" {
		scope.CampaignID = &qCampaignID
	}
	switch role {
	case model.RoleAdmin:
		if qRegionID != "" {
			scope.RegionID = &qRegionID
		}
	case model.RoleSupervisor:
		if userRegionID == nil || *userRegionID == "" {
			return scope, ErrRegionRequired
		}
		if qRegionID != "" && qRegionID != *userRegionID {
			return scope, ErrRegionForbidden
		}
		scope.RegionID = userRegionID
	case model.RoleExecutive:
		scope.TrafficManagerID = &userID
	default:
		scope.OwnerID = &userID
	}
	return scope, nil
}

// CampaignInScope reports whether a campaign belongs to a resolved scope.
// Used to authorize explicit campaign_id filters for non-admin roles.
func CampaignInScope(scope ReportScope, campaignRegionID *string, ownerID string, trafficManagerID *string) bool {
	if scope.RegionID != nil {
		if campaignRegionID == nil || *campaignRegionID != *scope.RegionID {
			return false
		}
	}
	if scope.TrafficManagerID != nil {
		if trafficManagerID == nil || *trafficManagerID != *scope.TrafficManagerID {
			return false
		}
	}
	if scope.OwnerID != nil && ownerID != *scope.OwnerID {
		return false
	}
	return true
}
