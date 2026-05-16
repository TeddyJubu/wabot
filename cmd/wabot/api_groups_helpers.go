package main

import (
	"go.mau.fi/whatsmeow/types"
)

func groupInfoToMap(g *types.GroupInfo) map[string]any {
	if g == nil {
		return map[string]any{}
	}
	participants := make([]map[string]any, 0, len(g.Participants))
	for _, p := range g.Participants {
		participants = append(participants, map[string]any{
			"jid":      p.JID.String(),
			"is_admin": p.IsAdmin,
			"is_super": p.IsSuperAdmin,
		})
	}
	return map[string]any{
		"jid":               g.JID.String(),
		"name":              g.GroupName.Name,
		"topic":             g.Topic,
		"participant_count": len(g.Participants),
		"announce":          g.GroupAnnounce.IsAnnounce,
		"locked":            g.GroupLocked.IsLocked,
		"created_at":        g.GroupCreated.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"participants":      participants,
	}
}
