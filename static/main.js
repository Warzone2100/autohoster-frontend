function nameFormatter(value, row) {
	let ret = `<div align="left" style="height:45px;">
<table cellspacing="0" cellpadding="0" style="margin: 0">
<tbody><tr><td class="rank-star">`;
	let Played = row.Played;
	let Won = row.Won;
	let Lost = row.Lost;
	let Elo = row.Elo;
	let EloDiff = row.EloDiff;
	let DisplayName = row.DisplayName !== undefined ? row.DisplayName : row.Name;
	let Account = row.Account !== undefined ? row.Account : row.AccountID;
	let Identity = row.Identity !== undefined ? row.Identity : row.Identity;
	let IdentityPubKey = row.IdentityPubKey;
	let url = "";
	if (IdentityPubKey !== undefined) {
		url = `href="/players/${IdentityPubKey}"`;
	}
	if(DisplayName !== undefined && DisplayName.length > 23) {
		DisplayName = DisplayName.slice(0, 20) + '...';
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
	let dn = document.createElement('text')
	dn.appendChild(document.createTextNode(DisplayName))
	ret += `<a ${url} class="text-nowrap">${dn.outerHTML}</a>`;
	if(Account <= 0) {
		ret += '<br><small class="text-muted class="text-nowrap"">not registered</small>';
	} else {
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

function renderPlayers() {
	let pls = document.querySelectorAll("div[loadPlayer]");
	for (const pl of pls) {
		let ob = JSON.parse(pl.attributes['loadplayer'].nodeValue);
		pl.outerHTML = nameFormatter(null, ob);
	}
}

function renderTimestamps() {
	let ts = document.querySelectorAll("time[datetime]");
	for (const t of ts) {
		let d = new Date(t.attributes['datetime'].nodeValue);
		t.textContent = d.toLocaleString();
	}
}
