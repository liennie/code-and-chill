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
