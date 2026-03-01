(function () {
	const KEY = 'darkmode';
	const html = document.documentElement;

	// small cookie helpers (name/value, days expiry)
	function getCookie(name) {
		const m = document.cookie.match('(?:^|; )' + name.replace(/([.*+?^${}()|[\]\\])/g, '\\$1') + '=([^;]*)');
		return m ? decodeURIComponent(m[1]) : null;
	}
	function setCookie(name, value, days) {
		try {
			let s = name + '=' + encodeURIComponent(value) + '; path=/; SameSite=Lax';
			if (days) s += '; Max-Age=' + (days * 24 * 60 * 60);
			document.cookie = s;
		} catch (e) { /* noop */ }
	}

	function applyDark(dark) {
		html.classList.toggle('dark', dark);
		const icon = document.querySelector('.dark-switch .icon');
		if (icon) icon.textContent = dark ? '🌙' : '☀️';
		// persist in a cookie for ~1 year
		setCookie(KEY, dark ? '1' : '0', 365);
	}

	function init() {
		const toggle = document.getElementById('darkmode-toggle');

		const saved = getCookie(KEY);
		if (saved !== null) {
			const on = (saved === '1');
			if (toggle) toggle.checked = on;
			applyDark(on);
		} else {
			const prefers = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
			if (toggle) toggle.checked = prefers;
			applyDark(prefers);
		}

		if (toggle) {
			if (toggle.getAttribute('data-init')) return;
			toggle.setAttribute('data-init', '1');
			toggle.addEventListener('change', function () {
				const on = !!toggle.checked;
				applyDark(on);
			});
		}
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', init);
	} else {
		init();
	}

	window.addEventListener('pageshow', function (ev) {
		if (ev.persisted) {
			init();
		}
	});
})();

document.addEventListener('click', function (ev) {
	try {
		if (ev.detail === 3 && ev.target) {
			let el = ev.target;
			// handle case where PRE contains a single CODE child and the click lands on CODE
			if (el.tagName === 'CODE' && el.parentElement && el.parentElement.tagName === 'PRE') {
				el = el.parentElement;
			}
			if (el.tagName === 'PRE') {
				const range = document.createRange();
				range.selectNodeContents(el);
				const sel = window.getSelection();
				sel.removeAllRanges();
				sel.addRange(range);
			}
		}
	} catch (e) { }
}, true);

(function () {
	let intervalId = null;

	function start() {
		const HINT_SEL = '.leftmenu .time.hint[data-unlock]';
		const nodes = Array.from(document.querySelectorAll(HINT_SEL));
		if (!nodes.length) return;

		function formatHMS(ms) {
			if (ms <= 0) return '00:00:00';
			const s = Math.ceil(ms / 1000);
			const hh = String(Math.floor(s / 3600)).padStart(2, '0');
			const mm = String(Math.floor((s % 3600) / 60)).padStart(2, '0');
			const ss = String(s % 60).padStart(2, '0');
			return `${hh}:${mm}:${ss}`;
		}

		function formatDHM(date) {
			const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
			const day = days[date.getDay()];
			const hours = String(date.getHours()).padStart(2, '0');
			const minutes = String(date.getMinutes()).padStart(2, '0');
			return `${day} ${hours}:${minutes}`;
		}

		function formatDMHM(date) {
			const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
				'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
			const month = months[date.getMonth()];
			const day = String(date.getDate()).padStart(2, '0');
			const hours = String(date.getHours()).padStart(2, '0');
			const minutes = String(date.getMinutes()).padStart(2, '0');
			return `${month} ${day} ${hours}:${minutes}`;
		}

		function updateCountdown() {
			const now = Date.now();
			const nowDate = new Date();

			for (let idx = nodes.length - 1; idx >= 0; idx--) {
				const el = nodes[idx];

				const iso = el.getAttribute('data-unlock');
				if (!iso) {
					nodes.splice(idx, 1);
					return;
				}

				const t = Date.parse(iso);
				if (isNaN(t)) {
					nodes.splice(idx, 1);
					return;
				}

				const diff = t - now;

				if (diff <= 0) {
					const li = el.closest('li');
					if (li && li.classList.contains('locked')) {
						li.classList.remove('locked');
						li.classList.add('unlocked');
					}

					const span = el.parentElement.closest('span');
					const link = document.createElement('a');
					for (const attr of span.attributes) {
						link.setAttribute(attr.name, attr.value);
					}
					while (span.childNodes.length) {
						link.appendChild(span.childNodes[0]);
					}
					span.parentNode.replaceChild(link, span);

					el.parentNode.removeChild(el);

					nodes.splice(idx, 1);
					return;
				}

				const unlockDate = new Date(t);
				const isFutureDay =
					unlockDate.getFullYear() !== nowDate.getFullYear() ||
					unlockDate.getMonth() !== nowDate.getMonth() ||
					unlockDate.getDate() !== nowDate.getDate();

				if (diff > 7 * 24 * 3600 * 1000) {
					// More than a week away
					el.textContent = formatDMHM(unlockDate);
				} else if (isFutureDay) {
					// Within a week but not today
					el.textContent = formatDHM(unlockDate);
				} else {
					// Same day
					el.textContent = formatHMS(diff);
				}
			};

			if (!nodes.length && intervalId) {
				clearInterval(intervalId);
			}
		}

		if (intervalId) clearInterval(intervalId);
		updateCountdown();
		intervalId = setInterval(updateCountdown, 1000);
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', start);
	} else {
		start();
	}

	window.addEventListener('pageshow', function (ev) {
		if (ev.persisted) {
			start();
		}
	});
})();

(function () {
	let intervalId = null;

	function start() {
		const HINT_SEL = 'em[data-timeout]';
		const nodes = Array.from(document.querySelectorAll(HINT_SEL));
		if (!nodes.length) return;

		function formatTimeout(ms) {
			if (ms <= 0) return '0s';
			const s = Math.ceil(ms / 1000);
			const dd = String(Math.floor(s / (24 * 60 * 60)))
			const hh = String(Math.floor(s % (24 * 60 * 60) / (60 * 60)));
			const mm = String(Math.floor((s % (60 * 60)) / 60));
			const ss = String(s % 60);

			if (dd > 0) {
				return `${dd}d ${hh.padStart(2, '0')}h ${mm.padStart(2, '0')}m ${ss.padStart(2, '0')}s`;
			}
			if (hh > 0) {
				return `${hh}h ${mm.padStart(2, '0')}m ${ss.padStart(2, '0')}s`;
			}
			if (mm > 0) {
				return `${mm}m ${ss.padStart(2, '0')}s`;
			}
			return `${ss}s`;
		}

		function updateCountdown() {
			const now = Date.now();
			const nowDate = new Date();

			for (let idx = nodes.length - 1; idx >= 0; idx--) {
				const el = nodes[idx];

				const iso = el.getAttribute('data-timeout');
				if (!iso) {
					nodes.splice(idx, 1);
					return;
				}

				const t = Date.parse(iso);
				if (isNaN(t)) {
					nodes.splice(idx, 1);
					return;
				}

				const diff = t - now;
				el.textContent = formatTimeout(diff);

				if (diff <= 0) {
					nodes.splice(idx, 1);
				}
			};

			if (!nodes.length && intervalId) {
				clearInterval(intervalId);
			}
		}

		if (intervalId) clearInterval(intervalId);
		updateCountdown();
		intervalId = setInterval(updateCountdown, 1000);
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', start);
	} else {
		start();
	}

	window.addEventListener('pageshow', function (ev) {
		if (ev.persisted) {
			start();
		}
	});
})();

(function () {
	function attachSpoilerHandlers() {
		const spoilers = document.querySelectorAll('span[data-spoiler]');
		spoilers.forEach(function (span) {
			const spoiler = span.getAttribute('data-spoiler');
			const placeholder = span.textContent || '********';
			const pad = span.getAttribute('data-pad');
			if (spoiler !== null) {
				span.addEventListener('click', function () {
					const revealed = span.classList.toggle('revealed');
					if (revealed) {
						switch (pad) {
							case 'start': span.textContent = spoiler.padStart(placeholder.length, ' '); break;
							case 'end': span.textContent = spoiler.padEnd(placeholder.length, ' '); break;
							default: span.textContent = spoiler;
						}
					} else {
						span.textContent = placeholder;
					}
				});
			}
		});
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', attachSpoilerHandlers);
	} else {
		attachSpoilerHandlers();
	}
})();
