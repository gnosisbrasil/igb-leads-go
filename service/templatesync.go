package service

import (
	"context"
	"log"

	"igb-leads-go/repository"
)

// SyncTemplates heals corrupted emojis at boot. Team edits are never
// reverted: outdated defaults refresh only via seed-defaults.
func SyncTemplates(ctx context.Context, campaigns *repository.CampaignRepository, templates *repository.TemplateRepository) {
	rows, err := campaigns.IDsAndObjectives(ctx)
	if err != nil {
		log.Printf("❌ Erro ao corrigir templates: %v", err)
		return
	}
	fixed := 0
	for _, row := range rows {
		objectives := row.Objectives
		if objectives == "" {
			objectives = "camara_publica"
		}
		for _, def := range DefaultsFor(objectives) {
			tpl, err := templates.ByCampaignAndKey(ctx, row.ID, def.Key)
			if err != nil {
				continue
			}
			if HasCorruptedEmojis(tpl.Content) {
				if err := templates.UpdateFields(ctx, tpl.ID, map[string]any{"content": def.Content}); err != nil {
					log.Printf("❌ Erro ao corrigir template %s: %v", tpl.ID, err)
					continue
				}
				fixed++
			}
		}
	}
	if fixed > 0 {
		log.Printf("✅ %d templates corrigidos (emojis corrompidos ou desatualizados)", fixed)
	} else {
		log.Print("✅ Templates OK (nenhum emoji corrompido encontrado)")
	}
}
