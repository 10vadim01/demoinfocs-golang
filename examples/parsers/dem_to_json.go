package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// Event represents a single event at a specific tick
type Event struct {
	Type    string                 `json:"type"`
	Data    map[string]interface{} `json:"data"`
	RawLine string                 `json:"raw_line"`
}

// TickData represents all data for a specific tick
type TickData struct {
	Tick   int     `json:"tick"`
	Events []Event `json:"events"`
}

// DemoData represents the complete demo data
type DemoData struct {
	MapName    string     `json:"map_name"`
	TickRate   int        `json:"tick_rate"`
	Duration   float64    `json:"duration"`
	TotalTicks int        `json:"total_ticks"`
	Ticks      []TickData `json:"ticks"`
}

// Position represents a 3D coordinate
type Position struct {
	X, Y, Z float64
}

// previousPositions stores the last known position for each player
var previousPositions = make(map[string]Position)

// tickEvents stores events for each tick
var tickEvents = make(map[int][]Event)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run demo_to_json.go <demo_file_path>")
		os.Exit(1)
	}

	demoPath := os.Args[1]

	f, err := os.Open(demoPath)
	checkError(err)
	defer f.Close()

	outPath := defaultOutPath(demoPath)
	outFile, err := os.Create(outPath)
	checkError(err)
	defer outFile.Close()

	targetName := "VadimkaYbivaet"

	p := demoinfocs.NewParser(f)
	defer p.Close()

	// Track demo properties
	var mapName string
	var tickRate int = 64 // Default
	var totalTicks int

	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_ServerInfo) {
		mapName = m.GetMapName()
	})

	p.RegisterEventHandler(func(e events.Kill) {
		tick := p.GameState().IngameTick()
		killerSide, killerName := playerSideName(e.Killer)
		victimSide, victimName := playerSideName(e.Victim)
		kx, ky, kz := playerPosXYZ(e.Killer)
		vx, vy, vz := playerPosXYZ(e.Victim)

		event := Event{
			Type: "kill",
			Data: map[string]interface{}{
				"killer_side": killerSide,
				"killer_name": killerName,
				"victim_side": victimSide,
				"victim_name": victimName,
				"weapon":      e.Weapon.String(),
				"killer_pos":  map[string]float64{"x": kx, "y": ky, "z": kz},
				"victim_pos":  map[string]float64{"x": vx, "y": vy, "z": vz},
			},
			RawLine: fmt.Sprintf("<kill><tick>%d</tick><killer><side>%s</side><name>%s</name><weapon>%v</weapon><pos>(%.1f,%.1f,%.1f)</pos></killer><victim><side>%s</side><name>%s</name><pos>(%.1f,%.1f,%.1f)</pos></victim></kill>",
				tick, killerSide, killerName, e.Weapon, kx, ky, kz, victimSide, victimName, vx, vy, vz),
		}
		addEventToTick(tick, event)
	})

	p.RegisterEventHandler(func(e events.RoundStart) {
		tick := p.GameState().IngameTick()
		gs := p.GameState()
		roundNumber := gs.TotalRoundsPlayed() + 1

		event := Event{
			Type: "round_start",
			Data: map[string]interface{}{
				"round":      roundNumber,
				"time_limit": e.TimeLimit,
				"objective":  e.Objective,
			},
			RawLine: fmt.Sprintf("<round_start><tick>%d</tick><round>%d</round><timeLimit>%d</timeLimit><objective>%s</objective></round_start>",
				tick, roundNumber, e.TimeLimit, e.Objective),
		}
		addEventToTick(tick, event)
	})

	p.RegisterEventHandler(func(e events.IsWarmupPeriodChanged) {
		tick := p.GameState().IngameTick()

		eventType := "warmup_start"
		if !e.NewIsWarmupPeriod {
			eventType = "warmup_end"
		}

		event := Event{
			Type: eventType,
			Data: map[string]interface{}{
				"is_warmup": e.NewIsWarmupPeriod,
			},
			RawLine: fmt.Sprintf("<%s><tick>%d</tick></%s>", eventType, tick, eventType),
		}
		addEventToTick(tick, event)
	})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		tick := p.GameState().IngameTick()
		gs := p.GameState()
		roundNumber := gs.TotalRoundsPlayed()
		winner := "none"
		reason := fmt.Sprintf("%d", e.Reason)

		switch e.Winner {
		case common.TeamTerrorists:
			winner = "T"
		case common.TeamCounterTerrorists:
			winner = "CT"
		}

		// Determine if target player's team won or lost
		targetTeam := ""
		targetWon := false
		for _, pl := range gs.Participants().Playing() {
			if isTargetPlayer(pl, targetName) {
				switch pl.Team {
				case common.TeamTerrorists:
					targetTeam = "T"
					targetWon = (e.Winner == common.TeamTerrorists)
				case common.TeamCounterTerrorists:
					targetTeam = "CT"
					targetWon = (e.Winner == common.TeamCounterTerrorists)
				}
				break
			}
		}

		result := "lost"
		if targetWon {
			result = "won"
		}

		event := Event{
			Type: "round_end",
			Data: map[string]interface{}{
				"round":       roundNumber,
				"winner":      winner,
				"reason":      reason,
				"target_team": targetTeam,
				"result":      result,
				"score_t":     gs.TeamTerrorists().Score(),
				"score_ct":    gs.TeamCounterTerrorists().Score(),
			},
			RawLine: fmt.Sprintf("<round_end><tick>%d</tick><round>%d</round><winner>%s</winner><reason>%s</reason><targetTeam>%s</targetTeam><result>%s</result><scoreT>%d</scoreT><scoreCT>%d</scoreCT></round_end>",
				tick, roundNumber, winner, reason, targetTeam, result, gs.TeamTerrorists().Score(), gs.TeamCounterTerrorists().Score()),
		}
		addEventToTick(tick, event)
	})

	p.RegisterEventHandler(func(e events.WeaponFire) {
		tick := p.GameState().IngameTick()
		side, name := playerSideName(e.Shooter)
		x, y, z := playerPosXYZ(e.Shooter)

		if isTargetPlayer(e.Shooter, targetName) {
			event := Event{
				Type: "shot",
				Data: map[string]interface{}{
					"side":      side,
					"name":      name,
					"weapon":    e.Weapon.String(),
					"position":  map[string]float64{"x": x, "y": y, "z": z},
					"location":  e.Shooter.LastPlaceName(),
					"inventory": formatInventory(e.Shooter),
				},
				RawLine: fmt.Sprintf("<shot><tick>%d</tick><side>%s</side><name>%s</name><weapon>%v</weapon><pos>(%.1f,%.1f,%.1f)</pos><location>%s</location><inv>%s</inv></shot>",
					tick, side, name, e.Weapon, x, y, z, e.Shooter.LastPlaceName(), formatInventory(e.Shooter)),
			}
			addEventToTick(tick, event)
		}
	})

	p.RegisterEventHandler(func(e events.PlayerHurt) {
		if isTargetPlayer(e.Attacker, targetName) || isTargetPlayer(e.Player, targetName) {
			tick := p.GameState().IngameTick()
			attSide, attName := playerSideName(e.Attacker)
			vicSide, vicName := playerSideName(e.Player)
			ax, ay, az := playerPosXYZ(e.Attacker)
			vx, vy, vz := playerPosXYZ(e.Player)

			event := Event{
				Type: "damage",
				Data: map[string]interface{}{
					"attacker_side": attSide,
					"attacker_name": attName,
					"victim_side":   vicSide,
					"victim_name":   vicName,
					"weapon":        e.Weapon.String(),
					"attacker_pos":  map[string]float64{"x": ax, "y": ay, "z": az},
					"victim_pos":    map[string]float64{"x": vx, "y": vy, "z": vz},
					"location":      e.Player.LastPlaceName(),
					"health":        e.Health,
					"armor":         e.Armor,
					"hp_damage":     e.HealthDamage,
					"armor_damage":  e.ArmorDamage,
					"hit_group":     fmt.Sprintf("%d", e.HitGroup),
				},
				RawLine: fmt.Sprintf("<damage><tick>%d</tick><attacker><side>%s</side><name>%s</name><weapon>%v</weapon><pos>(%.1f,%.1f,%.1f)</pos></attacker><victim><side>%s</side><name>%s</name><pos>(%.1f,%.1f,%.1f)</pos><location>%s</location><hp>%d</hp><armor>%d</armor><hpDamage>%d</hpDamage><armorDamage>%d</armorDamage><hitgroup>%v</hitgroup></victim></damage>",
					tick, attSide, attName, e.Weapon, ax, ay, az, vicSide, vicName, vx, vy, vz, e.Player.LastPlaceName(), e.Health, e.Armor, e.HealthDamage, e.ArmorDamage, e.HitGroup),
			}
			addEventToTick(tick, event)
		}
	})

	p.RegisterEventHandler(func(e events.FrameDone) {
		// if p.CurrentFrame()%10 != 0 {
		// 	return
		// }
		tick := p.GameState().IngameTick()
		for _, pl := range p.GameState().Participants().Playing() {
			if !isTargetPlayer(pl, targetName) {
				continue
			}
			pos := pl.Position()
			side, name := playerSideName(pl)
			helmet := 0
			if pl.HasHelmet() {
				helmet = 1
			}

			// Check if position has changed
			playerKey := fmt.Sprintf("%s_%s", side, name)
			currentPos := Position{X: pos.X, Y: pos.Y, Z: pos.Z}
			action := "move"

			if prevPos, exists := previousPositions[playerKey]; exists {
				if prevPos.X == currentPos.X && prevPos.Y == currentPos.Y && prevPos.Z == currentPos.Z {
					action = "stand"
				}
			}

			// Update previous position
			previousPositions[playerKey] = currentPos

			event := Event{
				Type: action,
				Data: map[string]interface{}{
					"side":      side,
					"name":      name,
					"position":  map[string]float64{"x": pos.X, "y": pos.Y, "z": pos.Z},
					"location":  pl.LastPlaceName(),
					"inventory": formatInventory(pl),
					"health":    pl.Health(),
					"armor":     pl.Armor(),
					"helmet":    helmet,
					"money":     pl.Money(),
				},
				RawLine: fmt.Sprintf("<%s><tick>%d</tick><player><side>%s</side><name>%s</name><pos>(%.1f,%.1f,%.1f)</pos><location>%s</location><inv>%s</inv><health>%d</health><armor>%d</armor><helmet>%d</helmet><money>%d</money></player></%s>",
					action, tick, side, name, pos.X, pos.Y, pos.Z, pl.LastPlaceName(), formatInventory(pl), pl.Health(), pl.Armor(), helmet, pl.Money(), action),
			}
			addEventToTick(tick, event)
		}
	})

	err = p.ParseToEnd()
	checkError(err)

	// Convert map to slice and sort by tick
	var ticks []TickData
	for tick, events := range tickEvents {
		ticks = append(ticks, TickData{
			Tick:   tick,
			Events: events,
		})
	}

	// Sort ticks by tick number
	sort.Slice(ticks, func(i, j int) bool {
		return ticks[i].Tick < ticks[j].Tick
	})

	// Create final data structure
	demoData := DemoData{
		MapName:    mapName,
		TickRate:   tickRate,
		Duration:   float64(totalTicks) / float64(tickRate),
		TotalTicks: totalTicks,
		Ticks:      ticks,
	}

	// Write JSON
	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(demoData)
	checkError(err)

	fmt.Printf("Demo data written to: %s\n", outPath)
	fmt.Printf("Map: %s, Ticks: %d, Events: %d\n", mapName, len(ticks), len(tickEvents))
}

func addEventToTick(tick int, event Event) {
	if tickEvents[tick] == nil {
		tickEvents[tick] = make([]Event, 0)
	}
	tickEvents[tick] = append(tickEvents[tick], event)
}

func isTargetPlayer(p *common.Player, target string) bool {
	if p == nil {
		return false
	}
	return strings.Contains(strings.ToLower(p.Name), strings.ToLower(target))
}

func formatInventory(p *common.Player) string {
	if p == nil {
		return "[]"
	}
	ws := p.Weapons()
	if len(ws) == 0 {
		return "[]"
	}
	names := make([]string, 0, len(ws))
	for _, w := range ws {
		if w == nil {
			continue
		}
		names = append(names, w.String())
	}
	return "[" + strings.Join(names, ", ") + "]"
}

func defaultOutPath(demoPath string) string {
	dir := filepath.Dir(demoPath)
	base := filepath.Base(demoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == base {
		name = base
	}
	return filepath.Join(dir, name+".json")
}

func playerSideName(p *common.Player) (string, string) {
	if p == nil {
		return "?", "?"
	}
	side := "?"
	switch p.Team {
	case common.TeamTerrorists:
		side = "T"
	case common.TeamCounterTerrorists:
		side = "CT"
	default:
		side = "?"
	}
	return side, p.Name
}

func playerPosXYZ(p *common.Player) (float64, float64, float64) {
	if p == nil {
		return 0, 0, 0
	}
	pos := p.Position()
	return pos.X, pos.Y, pos.Z
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
