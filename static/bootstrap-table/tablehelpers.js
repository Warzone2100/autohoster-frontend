function nameFormatter(value, row) {
	let ret = `<div align="left" style="height:45px;">
<table cellspacing="0" cellpadding="0" style="margin: 0">
<tbody><tr><td class="rank-star">`;
	Played = row.Played
	Won = row.Won
	Lost = row.Lost
	Hash = row.Hash !== undefined ? row.Hash : row.hash
	Name = row.Name
	Elo = row.Elo
	Account = row.Account !== undefined ? row.Account : row.AccountID
	EloDiff = row.EloDiff
	Identity = row.Identity !== undefined ? row.Identity : row.Identity
	if(Name.length > 23) {
		Name = Name.slice(0, 20) + '...';
	}
	if(Played > 4) {
		if(Elo > 1800) {
			ret += `<object class="rank rank-starGold"></object>`;
		} else if(Elo > 1550) {
			ret += `<object class="rank rank-starSilver"></object>`;
		} else if(Elo > 1400) {
			ret += `<object class="rank rank-starBronze"></object>`;
		}
	}
	ret += `</td><td rowspan="3" class="rank-medal">`;
	if(Played > 4) {
		if(Lost == 0) {
		} else if(Won >= 24 && Won/Lost > 6) {
			ret += `<object class="rank rank-medalGold"></object>`;
		} else if(Won >= 12 && Won/Lost > 4) {
			ret += `<object class="rank rank-medalDouble"></object>`;
		} else if(Won >= 6 && Won/Lost > 3) {
			ret += `<object class="rank rank-medalSilver"></object>`;
		}
	} else {
		ret += `<object class="rank rank-pacifier"></object>`;
	}
	ret += `</td><td rowspan="3" class="rank-link">`;
	// ret += `<a href="/players/${ID}" class="text-nowrap${Userid>0?' rank-name-checkmark':""}" title="Hash: ${Hash}">${Name}</a>`;
	if(Account <= 0) {
		ret += `<a href="/identities/${Identity}" class="text-nowrap">${Name}</a>`;
		ret += '<br><small class="text-muted class="text-nowrap"">not registered</small>';
	} else {
		ret += `<a href="/players/${Account}" class="text-nowrap">${Name}</a>`;
		ret += `<br>${Elo}`;
	}
	if(EloDiff != undefined && EloDiff != 0) {
		ret += "&nbsp;";
		if(EloDiff >= 1) {
			ret += "+";
		}
		ret += EloDiff;
	}
	ret += `</td></tr><tr><td class="rank-star">`;
	if(Played > 60) {
		ret += `<object class="rank rank-starGold"></object>`;
	} else if(Played > 30) {
		ret += `<object class="rank rank-starSilver"></object>`;
	} else if(Played > 10) {
		ret += `<object class="rank rank-starBronze"></object>`;
	}
	ret += `</td></tr><tr><td class="rank-star">`;
	if(Played > 4) {
		if(Won > 60) {
			ret += `<object class="rank rank-starGold"></object>`;
		} else if(Won > 30) {
			ret += `<object class="rank rank-starSilver"></object>`;
		} else if(Won > 10) {
			ret += `<object class="rank rank-starBronze"></object>`;
		}
	}
	ret += `</td></tr></tbody></table></div>`;
	return ret;
}
var defaultTableOptions = {
	cache: false,
	idField: "ID",
	pagination: true,
	pageSize: 50,
	pageNumber: 1,
	paginationLoop: false,
	showExtendedPagination: true,
	pageList: [10, 15, 25, 35, 50, 100, 500],
	buttonsPrefix: "btn btn-sm btn-primary",
	buttons: "buttons",
	classes: "table table-striped table-sm",
	search: true,
	showSearchButton: true,
	searchOnEnterKey: true,
	showSearchClearButton: true,
	escape: true,
	showFilterControlSwitch: true,
	filterControlVisible: false,
	stickyHeader: true,
	filterControl: true,
	showRefresh: true,
	toolbar: "#table-toolbar",
	sortOrder: "desc"
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
	return `<img class="icons icons-base${value}">`
}
function ScavengersSettingsFormatter(value, row) {
	return `<img class="icons icons-scav${value?"1":"0"}">`
}
function AlliancesSettingsFormatter(value, row) {
	return `<img class="icons icons-alliance${value == 3?"1":value}">`
}
function IDFormatter(value, row) {
	return `${row.ID} ${row.Calculated?"":"<text style=\"color:red;\">C</text>"} ${row.DebugTriggered?"<text class=\"rainbow\">D</text>":""}<br>${row.Version}`;
}
function TimeFormatter(value, row) {
	let igt = "in-game";
	if(row.GameTime > 200) {
		let seconds = Math.floor(row.GameTime / 1000);
		let minutes = Math.floor(seconds / 60);
		let hours = Math.floor(minutes / 60);
		seconds = seconds % 60;
		minutes = minutes % 60;
		igt = hours == 0 ? "" : hours + ":"
		igt += (hours != 0 && minutes < 10 ? '0' : '') + minutes + ":" + (seconds < 10 ? '0' : '') + seconds;
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
				s1.push(`<div class="row"><div class="col-sm">${nameFormatter(undefined, p)}</div></div>`);
			} else {
				s2.push(`<div class="row"><div class="col-sm">${nameFormatter(undefined, p)}</div></div>`);
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
	if(value === undefined) {
		return "Undefined!"
	}
	if(value === null) {
		return "NULL!"
	}
	return `<code title="${value}" class="hashshort">${value}</code>`
}
function winrateFormatter(value, row) {
	return (((row.Autowon+row.Autolost)==0?0:row.Autowon/(row.Autowon+row.Autolost))*100).toFixed(2) + "%"
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
	if ((ra.Autowon/(ra.Autowon+ra.Autolost+0.05)) > (rb.Autowon/(rb.Autowon+rb.Autolost+0.05))) return 1;
	if ((ra.Autowon/(ra.Autowon+ra.Autolost+0.05)) < (rb.Autowon/(rb.Autowon+rb.Autolost+0.05))) return -1;
	return 0;
}
function elo2Sorter(a, b, ra, rb) {
	if (ra.Userid <= 0 || rb.Userid <= 0) {
		return -1;
	}
	let d = ra.Elo2 - rb.Elo2;
	if (d < 0) {
		return -1;
	}
	return d > 0;
}
function timeplayedFormatter(value, row) {
	if(value === undefined) {
		return "???"
	} else if(value == 0) {
		return "0"
	} else {
		let hoursLeft = Math.floor(value / 3600);
		let min = Math.floor((value - hoursLeft * 3600) / 60);
		let secondsLeft = value - hoursLeft * 3600 - min * 60;
		secondsLeft = Math.round(secondsLeft * 100) / 100;
		let answer = hoursLeft< 10 ? "0" + hoursLeft : hoursLeft;
		answer += ":" + (min < 10 ? "0" + min : min);
		answer += ":" + (secondsLeft< 10 ? "0" + secondsLeft : secondsLeft);
		return answer;
	}
}
function lastgameFormatter(date) {
	let seconds = Math.floor(date);
	let interval = seconds / 31536000;
	let ret = "";
	let retn = 0
	if (interval > 1) {
		ret = Math.floor(interval) + "y";
		retn++;
	}
	interval = (seconds % 31536000) / 2592000;
	if (interval > 1) {
		ret = ret + " " + Math.floor(interval) + "m";
		retn++;
	}
	interval = (seconds % 2592000) / 86400;
	if (interval > 1 && retn <= 2) {
		ret = ret + " " + Math.floor(interval) + "d";
		retn++;
	}
	interval = (seconds % 86400) / 3600;
	if (interval > 1  && retn <= 2) {
		ret = ret + " " + Math.floor(interval) + "h";
		retn++;
	}
	interval = (seconds % 3600) / 60;
	if (interval > 1  && retn <= 2) {
		ret = ret + " " + Math.floor(interval) + "m";
		retn++;
	}
	if (seconds % 60 > 1 && retn <= 2) {
		ret = ret + " " + Math.floor(seconds % 60) + "s"
	}
	return `<text title='${date} seconds'>${ret}</text>`;
}
function mapNameFormatter(value, row) {
	if(row.Mod == 'none' || row.Mod == 'vanilla' || row.Mod == '' || row.Mod === undefined) {
		return value;
	}
	return value + '<br>' + row.Mod;
}
