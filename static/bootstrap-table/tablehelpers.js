function nameFormatter(value, row) {
	let ret = `<div align="left" style="height:45px;">
<table cellspacing="0" cellpadding="0" style="margin: 0">
<tbody><tr><td class="rank-star">`;
	Autoplayed = row.Autoplayed !== undefined ? row.Autoplayed : row.autoplayed
	Autowon = row.Autowon !== undefined ? row.Autowon : row.autowon
	Autolost = row.Autolost !== undefined ? row.Autolost : row.autolost
	Hash = row.Hash !== undefined ? row.Hash : row.hash
	Name = row.Name !== undefined ? row.Name : row.name
	Elo = row.Elo !== undefined ? row.Elo : row.elo
	Elo2 = row.Elo2 !== undefined ? row.Elo2 : row.elo2
	Userid = row.Userid !== undefined ? row.Userid : row.userid
	ID = row.ID !== undefined ? row.ID : row.id
	EloDiff = row.EloDiff !== undefined ? row.EloDiff : row.elodiff
	RatingDiff = row.RatingDiff !== undefined ? row.RatingDiff : row.ratingdiff
	if(Autoplayed > 4) {
		if(Elo > 1800) {
			ret += `<object class="rank rank-starGold"></object>`;
		} else if(Elo > 1550) {
			ret += `<object class="rank rank-starSilver"></object>`;
		} else if(Elo > 1400) {
			ret += `<object class="rank rank-starBronze"></object>`;
		}
	}
	ret += `</td><td rowspan="3" class="rank-medal">`;
	if(Autoplayed > 4) {
		if(Autolost == 0) {
		} else if(Autowon > 24 && Autowon/Autolost > 12) {
			ret += `<object class="rank rank-medalGold"></object>`;
		} else if(Autowon > 12 && Autowon/Autolost > 6) {
			ret += `<object class="rank rank-medalDouble"></object>`;
		} else if(Autowon > 6 && Autowon/Autolost > 3) {
			ret += `<object class="rank rank-medalSilver"></object>`;
		}
	} else {
		ret += `<object class="rank rank-pacifier"></object>`;
	}
	ret += `</td><td rowspan="3" class="rank-link">`;
	ret += `<a href="/players/${ID}" title="Hash: ${Hash}">${Name}</a>`;
	if(Userid > 0) {
		ret += `<text class="games-winner-name">✔</text>`;
	}
	ret += `<br>${Elo}`;
	if(EloDiff != undefined && EloDiff != 0) {
		ret += "&nbsp;";
		if(EloDiff >= 1) {
			ret += "+";
		}
		ret += EloDiff;
	}
	if(RatingDiff != undefined && RatingDiff != 0) {
		ret += "&nbsp;";
		if(RatingDiff >= 1) {
			ret += "+";
		}
		ret += RatingDiff;
	}
			// {{if avail "EloDiff" .}}
			// {{if not (eq .EloDiff 0)}}
			// 	({{if ge .EloDiff 1}}+{{end}}{{.EloDiff}})
			// 	{{end}}
			// {{end}}
			// {{if avail "RatingDiff" .}}{{if not (eq .RatingDiff 0)}}({{if ge .RatingDiff 1}}+{{end}}{{.RatingDiff}}){{end}}{{end}}
	ret += `</td></tr><tr><td class="rank-star">`;
	if(Autoplayed > 60) {
		ret += `<object class="rank rank-starGold"></object>`;
	} else if(Autoplayed > 30) {
		ret += `<object class="rank rank-starSilver"></object>`;
	} else if(Autoplayed > 10) {
		ret += `<object class="rank rank-starBronze"></object>`;
	}
	ret += `</td></tr><tr><td class="rank-star">`;
	if(Autoplayed > 4) {
		if(Autowon > 60) {
			ret += `<object class="rank rank-starGold"></object>`;
		} else if(Autowon > 30) {
			ret += `<object class="rank rank-starSilver"></object>`;
		} else if(Autowon > 10) {
			ret += `<object class="rank rank-starBronze"></object>`;
		}
	}
	ret += `</td></tr></tbody></table></div>`;
	return ret;
}
function BaseLevelSettingsFilters(value) {
	return ["0", "1", "2"];
}
function ScavengersSettingsFilters() {
	return [true, false];
}
function AlliancesSettingsFilters(value) {
	return ["1", "2", "3"];
}
function BaseLevelSettingsFormatter(value, row) {
	return `<img class="icons icons-base${row.BaseLevel}">`
}
function ScavengersSettingsFormatter(value, row) {
	return `<img class="icons icons-scav${row.Scavengers?"1":"0"}">`
}
function AlliancesSettingsFormatter(value, row) {
	return `<img class="icons icons-alliance${row.Alliances == 3?"1":row.Alliances}">`
}
function IDFormatter(value, row) {
	return `${row.ID}<br>${row.Calculated?"":"<text style=\"color:red;\">C</text>"} ${row.DebugTriggered?"<text class=\"rainbow\">D</text>":""}`;
}
function TimeFormatter(value, row) {
	let igt = "in-game";
	if(row.GameTime > 200) {
		let minutes = Math.floor(row.GameTime / 60000);
		let seconds = ((row.GameTime % 60000) / 1000).toFixed(0);
		igt = minutes + ":" + (seconds < 10 ? '0' : '') + seconds;
	}
	return value + `<br>` + igt;
}
function detailsBtn(value, row) {
	return row.Finished?`<a class="btn btn-primary text-nowrap" href="/games/${row.ID}">More</a>`:`<a href="/games/${row.ID}" class="btn btn-primary text-nowrap" type="button"><span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>&nbsp;In game</a>`;
}
function produceSides(row) {
	let s1 = [];
	let s2 = [];
	let prevteam = row.Players[0].Team;
	let switched = false;
	for(let i=0; i<row.Players.length; i++) {
		let p = row.Players[i];
		if(p.hash == "") {
			continue;
		}
		if(prevteam != row.Players[i].Team) {
			prevteam = row.Players[i].Team;
			switched = true;
		}
		if(p.Usertype == "winner") {
			s1.push(`<div class="row"><div class="col-sm games-winner-name">${nameFormatter(undefined, p)}</div></div>`);
		} else if(p.Usertype == "loser") {
			s2.push(`<div class="row"><div class="col-sm games-loser-name">${nameFormatter(undefined, p)}</div></div>`);
		} else if(p.Usertype == "spectator") {
			// s2.push(`<div class="row"><div class="col-sm games-loser-name">${nameFormatter(undefined, p)}</div></div>`);
		} else {
			if(switched) {
				s1.push(`<div class="row"><div class="col-sm">${nameFormatter(undefined, row.Players[i])}</div></div>`);
			} else {
				s2.push(`<div class="row"><div class="col-sm">${nameFormatter(undefined, row.Players[i])}</div></div>`);
			}
		}
	}
	return [s1, s2];
}
function playersFormatterA(value, row) {
	let sides = produceSides(row)
	let s = sides[0];
	let ret = `<table>`;
	for(let i=0; i<s.length; i++) {
		ret += `<tr><td>`;
		ret += s[i];
		ret += `</td></tr>`;
	}
	ret += `</table>`;
	return ret;
}
function playersFormatterB(value, row) {
	let sides = produceSides(row)
	let s = sides[1];
	let ret = `<table>`;
	for(let i=0; i<s.length; i++) {
		ret += `<tr><td>`;
		ret += s[i];
		ret += `</td></tr>`;
	}
	ret += `</table>`;
	return ret;
}
function hashFormatter(value, row) {
	return value.slice(0, 15) + '...'
}
function winrateFormatter(value, row) {
	return ((row.Autoplayed==0?0:row.Autowon/(row.Autolost+row.Autolost))*100).toFixed(2) + "%"
}
function rownumberFormatter(value, row, idx) {
	return idx+1
}
function rownumberStyler(value, row, idx) {
	if(idx == 0) {
		return {classes: 'leaderboardGold'}
	} else if(idx == 1) {
		return {classes: 'leaderboardSilver'}
	} else if(idx == 2) {
		return {classes: 'leaderboardBronze'}
	} else {
		return {}
	}
}
function winrateSorter(a, b, ra, rb) {
	if ((ra.Autowon/(ra.Autolost+ra.Autolost+0.05)) > (rb.Autowon/(rb.Autolost+ra.Autolost+0.05))) return 1;
	if ((ra.Autowon/(ra.Autolost+ra.Autolost+0.05)) < (rb.Autowon/(rb.Autolost+ra.Autolost+0.05))) return -1;
	return 0;
}
