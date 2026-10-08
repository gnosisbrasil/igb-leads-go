package repository

import "testing"

func TestAttachRegionDaily(t *testing.T) {
	regions := []RegionLeadStat{
		{ID: "r1", Name: "Norte", Campaigns: 2, Leads: 5},
		{ID: "r2", Name: "Sul", Campaigns: 1, Leads: 0},
	}
	daily := map[string][]DailyLeadCount{
		"r1": {{Date: "2026-10-01", Leads: 2}, {Date: "2026-10-02", Leads: 3}},
		"rX": {{Date: "2026-10-01", Leads: 9}}, // unknown region: ignored
	}
	got := AttachRegionDaily(regions, daily)
	if len(got[0].Daily) != 2 || got[0].Daily[1].Leads != 3 {
		t.Fatalf("r1 daily = %+v, want 2 entries ending in 3", got[0].Daily)
	}
	if got[1].Daily != nil {
		t.Fatalf("r2 daily = %+v, want nil", got[1].Daily)
	}
	if got[0].Name != "Norte" || got[1].Leads != 0 {
		t.Fatalf("other fields mutated: %+v", got)
	}
}
