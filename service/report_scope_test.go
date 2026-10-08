package service

import (
	"testing"

	"igb-leads-go/model"
)

func TestResolveReportScope(t *testing.T) {
	region := "r1"
	other := "r2"
	cases := []struct {
		name      string
		role      string
		region    *string
		campaignQ string
		regionQ   string
		wantErr   error
		check     func(ReportScope) bool
	}{
		{
			name: "admin sem filtro ve tudo", role: model.RoleAdmin,
			check: func(s ReportScope) bool {
				return s.CampaignID == nil && s.RegionID == nil && s.TrafficManagerID == nil && s.OwnerID == nil
			},
		},
		{
			name: "admin filtra regiao", role: model.RoleAdmin, regionQ: "r9",
			check: func(s ReportScope) bool { return s.RegionID != nil && *s.RegionID == "r9" },
		},
		{
			name: "supervisor trava na propria regiao", role: model.RoleSupervisor, region: &region,
			check: func(s ReportScope) bool { return s.RegionID != nil && *s.RegionID == "r1" },
		},
		{
			name: "supervisor sem regiao erro", role: model.RoleSupervisor, wantErr: ErrRegionRequired,
		},
		{
			name: "supervisor filtra outra regiao erro", role: model.RoleSupervisor, region: &region, regionQ: other, wantErr: ErrRegionForbidden,
		},
		{
			name: "supervisor filtra propria regiao ok", role: model.RoleSupervisor, region: &region, regionQ: region,
			check: func(s ReportScope) bool { return s.RegionID != nil && *s.RegionID == "r1" },
		},
		{
			name: "executivo trava no proprio id", role: model.RoleExecutive,
			check: func(s ReportScope) bool { return s.TrafficManagerID != nil && *s.TrafficManagerID == "u1" },
		},
		{
			name: "usuario trava no proprio id", role: model.RoleUser,
			check: func(s ReportScope) bool { return s.OwnerID != nil && *s.OwnerID == "u1" },
		},
		{
			name: "filtro campanha passa junto", role: model.RoleAdmin, campaignQ: "c1",
			check: func(s ReportScope) bool { return s.CampaignID != nil && *s.CampaignID == "c1" },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveReportScope(tc.role, "u1", tc.region, tc.campaignQ, tc.regionQ)
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("erro = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if tc.check != nil && !tc.check(got) {
				t.Fatalf("escopo incorreto: %+v", got)
			}
		})
	}
}

func TestCampaignInScope(t *testing.T) {
	region := "r1"
	other := "r2"
	if !CampaignInScope(ReportScope{}, &other, "x", nil) {
		t.Fatal("escopo vazio (admin) deve aceitar tudo")
	}
	if !CampaignInScope(ReportScope{RegionID: &region}, &region, "x", nil) {
		t.Fatal("mesma regiao deve aceitar")
	}
	if CampaignInScope(ReportScope{RegionID: &region}, &other, "x", nil) {
		t.Fatal("outra regiao deve rejeitar")
	}
	if CampaignInScope(ReportScope{RegionID: &region}, nil, "x", nil) {
		t.Fatal("campanha sem regiao deve rejeitar escopo regional")
	}
	tm := "u1"
	if !CampaignInScope(ReportScope{TrafficManagerID: &tm}, nil, "x", &tm) {
		t.Fatal("executivo dono deve aceitar")
	}
	if CampaignInScope(ReportScope{TrafficManagerID: &tm}, nil, "x", nil) {
		t.Fatal("executivo nao-dono deve rejeitar")
	}
	if !CampaignInScope(ReportScope{OwnerID: &tm}, nil, "u1", nil) {
		t.Fatal("dono deve aceitar")
	}
	if CampaignInScope(ReportScope{OwnerID: &tm}, nil, "outro", nil) {
		t.Fatal("nao-dono deve rejeitar")
	}
}

func TestParseDateRange(t *testing.T) {
	r := ParseDateRange("2026-01-10", "2026-01-12")
	if !r.HasFrom || !r.HasTo {
		t.Fatal("deveria ter ambos os limites")
	}
	if r.From.Day() != 10 || r.To.Day() != 13 {
		t.Fatalf("limites errados: %v %v", r.From, r.To)
	}
	r = ParseDateRange("", "")
	if r.HasFrom || r.HasTo {
		t.Fatal("vazio deveria ser ilimitado")
	}
	r = ParseDateRange("invalida", "2026-01-12")
	if r.HasFrom || !r.HasTo {
		t.Fatal("from invalido deveria ser ignorado")
	}
	r = ParseDateRange("2026-01-12", "2026-01-10")
	if !r.From.Before(r.To) {
		t.Fatal("range invertido deveria normalizar")
	}
}
