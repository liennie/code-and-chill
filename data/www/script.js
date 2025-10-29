(function () {
	const KEY = 'woc:darkmode';
	const html = document.documentElement;
	const toggle = document.getElementById('darkmode-toggle');

	function applyDark(dark) {
		html.classList.toggle('dark', !!dark);
		if (toggle) toggle.checked = !!dark;
		const icon = document.querySelector('.dark-switch .icon');
		if (icon) icon.textContent = dark ? '🌙' : '☀️';
	}

	const saved = localStorage.getItem(KEY);
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
			try { localStorage.setItem(KEY, on ? '1' : '0'); } catch (e) { }
		});
	}
})();

document.addEventListener('click', function (ev) {
	try {
		if (ev.detail === 3 && ev.target && ev.target.tagName === 'PRE') {
			const el = ev.target;
			const range = document.createRange();
			range.selectNodeContents(el);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
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

	function updateCountdown() {
		const now = Date.now();
		nodes.forEach(el => {
			const iso = el.getAttribute('data-unlock');
			if (!iso) return;
			let t = Date.parse(iso);
			if (isNaN(t)) return;
			const diff = t - now;
			if (diff > 24 * 3600 * 1000) {
				el.textContent = '';
			} else if (diff <= 0) {
				el.textContent = '';
				// toggle classes on closest li
				const li = el.closest('li');
				if (li && li.classList.contains('locked')) {
					li.classList.remove('locked');
					li.classList.add('unlocked');
				}
			} else {
				el.textContent = formatHMS(diff);
			}
		});
	}

	updateCountdown();
	setInterval(updateCountdown, 1000);
})();
