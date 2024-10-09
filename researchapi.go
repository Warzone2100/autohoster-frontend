package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func APIgetResearchlogData(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gid := params["gid"]
	var j []map[string]any
	err := dbpool.QueryRow(context.Background(), `SELECT coalesce(research_log, '[]')::jsonb FROM games WHERE id = $1`, gid).Scan(&j)
	if err != nil {
		if err == pgx.ErrNoRows {
			return http.StatusNoContent, nil
		}
		return 500, err
	}
	for i := range j {
		for k, v := range j[i] {
			if k == "name" {
				j[i][k] = getResearchName(v.(string))
				j[i]["id"] = v.(string)
			}
		}
	}
	return 200, j
}

func APIgetResearchSummary(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gid := params["gid"]
	var researchLog []resEntry
	var players []Player
	var settingAlliance int
	err := dbpool.QueryRow(context.Background(), `SELECT
	coalesce(research_log, '[]')::jsonb,
	json_agg(json_build_object(
		'Position', p.position,
		'Name', i.name,
		'Team', p.team,
		'Usertype', p.usertype,
		'Color', p.color,
		'Identity', i.id,
		'IdentityPubKey', encode(i.pkey, 'hex'),
		'Account', a.id,
		'DisplayName', coalesce(i.name, a.display_name),
		'Rating', (select r from rating as r where r.category = g.display_category and r.account = i.account),
		'Props', p.props
	))::jsonb,
	setting_alliance
FROM games as g
JOIN players as p on g.id = p.game
JOIN identities as i on p.identity = i.id
LEFT JOIN accounts as a on a.id = i.account
WHERE g.id = $1
GROUP BY 1, 3`, gid).Scan(&researchLog, &players, &settingAlliance)
	if err != nil {
		if err == pgx.ErrNoRows {
			return http.StatusNoContent, nil
		}
		return 500, err
	}

	isShared := settingAlliance == 2

	teams := []struct {
		index     int
		positions []int
	}{}

	for _, pl := range players {
		tf := -1
		for t := range teams {
			if teams[t].index == pl.Team {
				tf = t
				break
			}
		}
		if tf == -1 {
			teams = append(teams, struct {
				index     int
				positions []int
			}{
				index:     pl.Team,
				positions: []int{pl.Position},
			})
		} else {
			teams[tf].positions = append(teams[tf].positions, pl.Position)
		}
	}

	topTimes := map[string]resEntry{}

	for _, v := range researchLog {
		tt, ok := topTimes[v.Name]
		if ok {
			if tt.Time >= v.Time {
				topTimes[v.Name] = v
			}
		} else {
			topTimes[v.Name] = v
		}
	}

	renderPaths := [][]string{
		{
			"Synaptic",
			"R-Struc-Research-Module",
			"R-Struc-Research-Upgrade01",
			"R-Struc-Research-Upgrade02",
			"R-Struc-Research-Upgrade03",
			"R-Struc-Research-Upgrade04",
			"R-Struc-Research-Upgrade05",
			"R-Struc-Research-Upgrade06",
			"R-Struc-Research-Upgrade07",
			"R-Struc-Research-Upgrade08",
			"R-Struc-Research-Upgrade09",
		}, {
			"AlloysBorgs",
			"R-Cyborg-Metals01",
			"R-Cyborg-Metals02",
			"R-Cyborg-Metals03",
			"R-Cyborg-Metals04",
			"R-Cyborg-Metals05",
			"R-Cyborg-Metals06",
			"R-Cyborg-Metals07",
			"R-Cyborg-Metals08",
			"R-Cyborg-Metals09",
		}, {
			"AlloysTanks",
			"R-Vehicle-Metals01",
			"R-Vehicle-Metals02",
			"R-Vehicle-Metals03",
			"R-Vehicle-Metals04",
			"R-Vehicle-Metals05",
			"R-Vehicle-Metals06",
			"R-Vehicle-Metals07",
			"R-Vehicle-Metals08",
			"R-Vehicle-Metals09",
		}, {
			"Power",
			"R-Struc-PowerModuleMk1",
			"R-Struc-Power-Upgrade01",
			"R-Struc-Power-Upgrade01b",
			"R-Struc-Power-Upgrade01c",
			"R-Struc-Power-Upgrade02",
			"R-Struc-Power-Upgrade03",
			"R-Struc-Power-Upgrade03a",
		}, {
			"Body",
			"R-Vehicle-Body01",
			"R-Vehicle-Body05",
			"R-Vehicle-Body11",
			"R-Vehicle-Body04",
			"R-Vehicle-Body08",
			"R-Vehicle-Body12",
			"R-Vehicle-Body02",
			"R-Vehicle-Body06",
			"R-Vehicle-Body09",
			"R-Vehicle-Body03",
			"R-Vehicle-Body07",
			"R-Vehicle-Body10",
			"R-Vehicle-Body13",
			"R-Vehicle-Body14",
		}, {
			"Cannon/Rail",
			"R-Wpn-Cannon1Mk1",
			"R-Wpn-Cannon-Damage01",
			"R-Wpn-Cannon-Damage02",
			"R-Wpn-Cannon2Mk1",
			"R-Wpn-Cannon-Accuracy01",
			"R-Wpn-Cannon-Damage03",
			"R-Wpn-Cannon4AMk1",
			"R-Wpn-Cannon-Damage04",
			"R-Wpn-Cannon-ROF01",
			"R-Wpn-Cannon-Accuracy02",
			"R-Wpn-Cannon-Damage05",
			"R-Wpn-Cannon5",
			"R-Cyborg-Hvywpn-Mcannon",
			"R-Wpn-Cannon-ROF02",
			"R-Cyborg-Hvywpn-Acannon",
			"R-Cyborg-Hvywpn-HPV",
			"R-Wpn-Cannon-Damage06",
			"R-Wpn-Cannon3Mk1",
			"R-Wpn-Cannon-ROF03",
			"R-Wpn-Cannon-Damage07",
			"R-Wpn-Cannon-ROF04",
			"R-Wpn-Cannon-Damage08",
			"R-Wpn-RailGun01",
			"R-Wpn-Cannon-ROF05",
			"R-Wpn-Cannon-Damage09",
			"R-Wpn-Rail-Damage01",
			"R-Wpn-Cannon-ROF06",
			"R-Wpn-Rail-Accuracy01",
			"R-Wpn-Rail-Damage02",
			"R-Wpn-Rail-ROF01",
			"R-Wpn-Rail-ROF02",
			"R-Wpn-RailGun02",
			"R-Cyborg-Hvywpn-RailGunner",
			"R-Wpn-Rail-Damage03",
			"R-Wpn-Rail-ROF03",
			"R-Wpn-RailGun03",
		}, {
			"Rockets",
			"R-Wpn-Rocket05-MiniPod",
			"R-Wpn-Rocket-Damage01",
			"R-Wpn-Rocket-Damage02",
			"R-Wpn-Rocket02-MRL",
			"R-Wpn-Rocket-ROF01",
			"R-Wpn-Rocket-Accuracy01",
			"R-Wpn-Rocket-Damage03",
			"R-Wpn-Rocket01-LtAT",
			"R-Wpn-Rocket-ROF02",
			"R-Wpn-Rocket-Damage04",
			"R-Wpn-Rocket-Accuracy02",
			"R-Wpn-Rocket-Damage05",
			"R-Wpn-Rocket-ROF03",
			"R-Wpn-RocketSlow-Accuracy01",
			"R-Wpn-Rocket02-MRLHvy",
			"R-Wpn-Rocket-Damage06",
			"R-Wpn-Rocket07-Tank-Killer",
			"R-Wpn-RocketSlow-Accuracy02",
			"R-Cyborg-Hvywpn-TK",
			"R-Wpn-Rocket-Damage07",
			"R-Wpn-Rocket-Damage08",
			"R-Wpn-Missile2A-T",
			"R-Wpn-Rocket-Damage09",
			"R-Cyborg-Hvywpn-A-T",
			"R-Wpn-Missile-ROF01",
			"R-Wpn-MdArtMissile",
			"R-Wpn-Missile-Damage01",
			"R-Wpn-Missile-Accuracy01",
			"R-Wpn-Missile-ROF02",
			"R-Wpn-Missile-Damage02",
			"R-Wpn-Missile-Accuracy02",
			"R-Wpn-Missile-ROF03",
			"R-Wpn-Missile-Damage03",
		}, {
			"MG",
			"R-Wpn-MG1Mk1",
			"R-Wpn-MG-Damage01",
			"R-Wpn-MG-Damage02",
			"R-Wpn-MG2Mk1",
			"R-Wpn-MG3Mk1",
			"R-Wpn-MG-Damage04",
			"R-Wpn-MG-ROF01",
			"R-Wpn-MG-ROF02",
			"R-Wpn-MG-Damage05",
			"R-Wpn-MG4",
			"R-Wpn-MG-Damage06",
			"R-Wpn-MG-ROF03",
			"R-Wpn-MG-Damage07",
			"R-Wpn-MG5",
			"R-Wpn-MG-Damage08",
			"R-Wpn-MG-Damage09",
			"R-Wpn-MG-Damage10",
		}, {
			"AA",
			"R-Wpn-AAGun03",
			"R-Wpn-Sunburst",
			"R-Wpn-AAGun01",
			"R-Defense-AASite-QuadMg1",
			"R-Defense-Sunburst",
			"R-Defense-AASite-QuadBof",
			"R-Wpn-AAGun04",
			"R-Defense-AASite-QuadRotMg",
			"R-Wpn-AAGun02",
			"R-Defense-AASite-QuadBof02",
			"R-Wpn-Missile-LtSAM",
			"R-Defense-SamSite1",
			"R-Wpn-AALaser",
			"R-Defense-AA-Laser",
			"R-Wpn-Missile-HvSAM",
			"R-Defense-SamSite2",
		}, {
			"Flamer",
			"R-Wpn-Flamer01Mk1",
			"R-Wpn-Flamer-Damage01",
			"R-Wpn-Flamer-Damage02",
			"R-Wpn-Flamer-ROF01",
			"R-Wpn-Flamer-Damage03",
			"R-Wpn-Flamer-Damage04",
			"R-Wpn-Flame2",
			"R-Wpn-Flamer-Damage05",
			"R-Wpn-Flamer-ROF02",
			"R-Wpn-Flamer-Damage06",
			"R-Wpn-Flamer-ROF03",
			"R-Wpn-Plasmite-Flamer",
			"R-Wpn-Flamer-Damage07",
			"R-Wpn-Flamer-Damage08",
			"R-Wpn-Flamer-Damage09",
		}, {
			"Arty",
			"R-Wpn-Mortar01Lt",
			"R-Defense-MortarPit",
			"R-Wpn-Mortar-Damage01",
			"R-Wpn-Mortar-Acc01",
			"R-Wpn-Mortar-Damage02",
			"R-Wpn-Mortar-Acc02",
			"R-Wpn-Mortar-Damage03",
			"R-Wpn-Mortar02Hvy",
			"R-Wpn-Mortar3",
			"R-Defense-HvyMor",
			"R-Defense-RotMor",
			"R-Wpn-Mortar-ROF01",
			"R-Wpn-Mortar-Acc03",
			"R-Wpn-Mortar-Incendiary",
			"R-Defense-MortarPit-Incendiary",
			"R-Wpn-Mortar-ROF02",
			"R-Wpn-Mortar-Damage04",
			"R-Wpn-Rocket06-IDF",
			"R-Wpn-Mortar-ROF03",
			"R-Defense-IDFRocket",
			"R-Wpn-Mortar-Damage05",
			"R-Wpn-HowitzerMk1",
			"R-Wpn-Mortar-ROF04",
			"R-Defense-Howitzer",
			"R-Wpn-Howitzer-Damage01",
			"R-Wpn-Mortar-Damage06",
			"R-Wpn-Howitzer-Accuracy01",
			"R-Wpn-Howitzer-Incendiary",
			"R-Wpn-Howitzer03-Rot",
			"R-Wpn-Howitzer-Damage02",
			"R-Defense-Howitzer-Incendiary",
			"R-Defense-RotHow",
			"R-Wpn-Howitzer-Accuracy02",
			"R-Wpn-Howitzer-Damage03",
			"R-Wpn-Howitzer-Accuracy03",
			"R-Wpn-HvyHowitzer",
			"R-Defense-HvyHowitzer",
			"R-Wpn-Howitzer-Damage04",
			"R-Wpn-Howitzer-ROF01",
			"R-Wpn-Howitzer-Damage05",
			"R-Wpn-Howitzer-ROF02",
			"R-Wpn-HeavyPlasmaLauncher",
			"R-Defense-HeavyPlasmaLauncher",
			"R-Wpn-HvArtMissile",
			"R-Wpn-Howitzer-Damage06",
			"R-Wpn-Howitzer-ROF03",
			"R-Defense-HvyArtMissile",
			"R-Wpn-Howitzer-ROF04",
		},
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].Position < players[j].Position
	})

	findResTime := func(key string, pos int) int {
		for _, v := range researchLog {
			if v.Name == key && int(v.Position) == pos {
				return int(v.Time)
			}
		}
		return -1
	}
	findTeamResTime := func(key string, pos []int) int {
		for _, v := range researchLog {
			if v.Name == key && slices.Contains(pos, int(v.Position)) {
				return int(v.Time)
			}
		}
		return -1
	}

	ret := `<style>
	.rs td {
		border: solid 1px;
		padding: 2px;
	}
	.rs {
		border-collapse: separate;
		border-spacing: 0px;
	}
	</style>
	<script>
	function rsToggle(id) {
		let els = document.querySelectorAll(id);
		console.log(els);
		for (const el of els) {
			if (el.style.display === "none") {
				el.style.display = "table-row";
			} else {
				el.style.display = "none";
			}
		}
	}
	</script>
	`
	ret += `<table class="rs">`
	for i, v := range renderPaths {
		ret += `<tr>`
		ret += fmt.Sprintf(`<td><a onclick="rsToggle('.rsPath%d');">👁</a></td>`, i)
		ret += `<td>` + v[0] + `</td>`
		if isShared {
			for _, t := range teams {
				ret += fmt.Sprintf(`<td>%c</td>`, "ABCDEFGHIJKLM"[t.index])
			}
		} else {
			for _, pl := range players {
				ret += fmt.Sprintf(`<td class="wz-color-background-%d">%s</td>`, pl.Color, pl.DisplayName)
			}
		}
		ret += `</tr>`
		for _, r := range v[1:] {
			if _, ok := topTimes[r]; !ok {
				continue
			}
			ret += fmt.Sprintf(`<tr class="rsPath%d" style="display: none;">`, i)
			ret += `<td><a href="https://betaguide.wz2100.net/research.html?details_id=` + r + `">
	<img src="https://betaguide.wz2100.net/img/data_icons/Research/` + getResearchName(r) + `.gif"></a></td>`
			ret += `<td><a href="https://betaguide.wz2100.net/research.html?details_id=` + r + `">` + getResearchName(r) + `<br>` + r + `</a></td>`

			if isShared {
				for t := range teams {
					tRes := findTeamResTime(r, teams[t].positions)
					tcont := `∅`
					tcol := ``
					if tRes != -1 {
						tcont = GameTimeToStringI(tRes)
						tcol = ` style="color: green;" `
						if float64(tRes)-topTimes[r].Time > 16000 {
							tcol = ` style="color: darkorange;" `
						}
						if float64(tRes)-topTimes[r].Time > 31000 {
							tcol = ` style="color: red;" `
						}
					}
					ret += fmt.Sprintf(`<td %s >%v</td>`, tcol, tcont)
				}
			} else {
				for _, pl := range players {
					tRes := findResTime(r, pl.Position)
					tcont := `∅`
					tcol := ``
					if tRes != -1 {
						tcont = GameTimeToStringI(tRes)
						tcol = ` style="color: green;" `
						if float64(tRes)-topTimes[r].Time > 16000 {
							tcol = ` style="color: darkorange;" `
						}
						if float64(tRes)-topTimes[r].Time > 31000 {
							tcol = ` style="color: red;" `
						}
					}
					ret += fmt.Sprintf(`<td %s >%v</td>`, tcol, tcont)
				}
			}
			ret += `</tr>`
		}
	}
	ret += `</table>`

	w.WriteHeader(200)
	w.Write([]byte(ret))
	return 0, nil
}

type resEntry struct {
	Name     string  `json:"name"`
	Position float64 `json:"position"`
	Time     float64 `json:"time"`
}

var (
	researchClassification []map[string]string
)

func LoadClassification() (ret []map[string]string, err error) {
	var content []byte
	content, err = os.ReadFile(cfg.GetDSString("classification.json", "researchClassification"))
	if err != nil {
		return
	}
	err = json.Unmarshal(content, &ret)
	return
}

// CountClassification in: classification, research out: position[research[time]]
func CountClassification(resl []resEntry) (ret map[int]map[string]int) {
	cl := map[string]string{}
	ret = map[int]map[string]int{}
	for _, b := range researchClassification {
		cl[b["name"]] = b["Subclass"]
	}
	for _, b := range resl {
		if b.Time < 10 {
			continue
		}
		j, f := cl[b.Name]
		if f {
			_, ff := ret[int(b.Position)]
			if !ff {
				ret[int(b.Position)] = map[string]int{}
			}
			_, ff = ret[int(b.Position)][j]
			if ff {
				ret[int(b.Position)][j]++
			} else {
				ret[int(b.Position)][j] = 1
			}
		}
	}
	return
}

func getPlayerClassifications(pid int) (total, recent map[string]int, err error) {
	total = map[string]int{}
	recent = map[string]int{}
	rows, err := dbpool.Query(context.Background(),
		`SELECT coalesce(id, -1), coalesce(researchlog, ''), array_position(players, $1)-1, coalesce(timestarted, now())
		FROM games 
		WHERE 
			$1 = any(players)
			AND (array_position(players, -1)-1 = 2 OR alliancetype = 3)
			AND finished = true 
			AND calculated = true 
			AND hidden = false 
			AND deleted = false 
			AND id > 1032
		ORDER BY id desc`, pid)
	if err != nil {
		if err == pgx.ErrNoRows {
			return
		}
		return
	}
	type gameResearch struct {
		gid       int
		playerpos int
		when      time.Time
		research  string
		cl        map[int]map[string]int
	}
	games := []gameResearch{}
	for rows.Next() {
		g := gameResearch{}
		err = rows.Scan(&g.gid, &g.research, &g.playerpos, &g.when)
		if err != nil {
			return
		}
		games = append(games, g)
	}
	for i, g := range games {
		var resl []resEntry
		err = json.Unmarshal([]byte(g.research), &resl)
		if err != nil {
			log.Print(err.Error())
			log.Print(spew.Sdump(g))
			continue
		}
		games[i].cl = CountClassification(resl)
		for v, c := range games[i].cl[g.playerpos] {
			if val, ok := total[v]; ok {
				total[v] = val + c
			} else {
				total[v] = c
			}
		}
		if i < 20 {
			for v, c := range games[i].cl[g.playerpos] {
				if val, ok := recent[v]; ok {
					recent[v] = val + c
				} else {
					recent[v] = c
				}
			}
		}
	}
	err = nil
	return
}

func APIresearchClassification(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	pids := params["pid"]
	pid, err := strconv.Atoi(pids)
	if err != nil {
		return 400, nil
	}
	a, b, err := getPlayerClassifications(pid)
	_ = a
	_ = b
	_ = err
	return 200, a
}
