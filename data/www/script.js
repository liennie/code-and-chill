(function () {
	const KEY = 'darkmode';
	const html = document.documentElement;
	const toggle = document.getElementById('darkmode-toggle');

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
		html.classList.toggle('dark', !!dark);
		if (toggle) toggle.checked = !!dark;
		const icon = document.querySelector('.dark-switch .icon');
		if (icon) icon.textContent = dark ? '🌙' : '☀️';
		// persist in a cookie for ~1 year
		setCookie(KEY, dark ? '1' : '0', 365);
	}

	const saved = getCookie(KEY);
	if (saved !== null) {
		applyDark(saved === '1');
	} else {
		const prefers = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
		applyDark(prefers);
	}

	if (toggle) {
		toggle.addEventListener('change', function () {
			const on = !!toggle.checked;
			applyDark(on);
		});
	}
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
	const HINT_SEL = '.leftmenu .hint[data-unlock]';
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
		const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun",
			"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
		const month = months[date.getMonth()];
		const day = String(date.getDate()).padStart(2, '0');
		const hours = String(date.getHours()).padStart(2, '0');
		const minutes = String(date.getMinutes()).padStart(2, '0');
		return `${month} ${day} ${hours}:${minutes}`;
	}

	function updateCountdown() {
		const now = Date.now();
		const nowDate = new Date();

		nodes.forEach(el => {
			const iso = el.getAttribute('data-unlock');
			if (!iso) return;

			const t = Date.parse(iso);
			if (isNaN(t)) return;

			const diff = t - now;

			if (diff <= 0) {
				el.textContent = '';
				const li = el.closest('li');
				if (li && li.classList.contains('locked')) {
					li.classList.remove('locked');
					li.classList.add('unlocked');
				}
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
		});
	}

	updateCountdown();
	setInterval(updateCountdown, 1000);
})();
