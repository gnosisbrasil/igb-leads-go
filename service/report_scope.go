package service

import (
	"errors"
	"time"

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

// DateRange bounds lead aggregates. From is inclusive, To is exclusive
// (callers add a day to include the whole end date).
type DateRange struct {
	From, To       time.Time
	HasFrom, HasTo bool
}

// ParseDateRange parses YYYY-MM-DD bounds. Empty or invalid values mean
// unbounded on that side; a inverted range swaps to stay valid.
func ParseDateRange(fromStr, toStr string) DateRange {
	var r DateRange
	if t, err := time.Parse("2006-01-02", fromStr); err == nil {
		r.From, r.HasFrom = t, true
	}
	if t, err := time.Parse("2006-01-02", toStr); err == nil {
		r.To, r.HasTo = t.Add(24*time.Hour), true
	}
	if r.HasFrom && r.HasTo && !r.From.Before(r.To) {
		r.From, r.To = r.To.Add(-24*time.Hour), r.From.Add(24*time.Hour)
	}
	return r
}
