(function () {
	function startLeaderboardChartControls() {
		const controls = document.getElementById('leaderboard-chart-controls');
		if (!controls) {
			return;
		}

		const slider = document.getElementById('leaderboard-chart-at');
		const nowButton = document.getElementById('leaderboard-chart-now');
		const valueOutput = document.getElementById('leaderboard-chart-at-value');
		const chartHost = document.getElementById('leaderboard-chart-host');
		const chartSrc = controls.getAttribute('data-chart-src');
		if (!slider || !nowButton || !valueOutput || !chartHost || !chartSrc) {
			return;
		}

		const min = Number(slider.min);
		const max = Number(slider.max);
		if (!Number.isFinite(min) || !Number.isFinite(max) || max < min) {
			return;
		}

		const clamp = (value) => Math.min(max, Math.max(min, value));
		const formatAt = (unixSeconds) => {
			return new Date(unixSeconds * 1000).toISOString();
		};

		const ensureChartImage = () => {
			const currentImage = chartHost.querySelector('img[data-admin-chart="1"]');
			if (currentImage) {
				return currentImage;
			}

			const image = document.createElement('img');
			image.setAttribute('data-admin-chart', '1');
			image.alt = 'leaderboard chart';
			image.decoding = 'async';
			chartHost.replaceChildren(image);
			return image;
		};

		const render = () => {
			const atUnix = clamp(Number(slider.value));
			slider.value = String(atUnix);

			const atRFC3339 = formatAt(atUnix);
			const date = new Date(atUnix * 1000);
			valueOutput.textContent = `${date.toLocaleString()} (${atRFC3339})`;

			const image = ensureChartImage();
			image.src = `${chartSrc}?at=${encodeURIComponent(atRFC3339)}`;
		};

		slider.value = String(clamp(Number(slider.value)));
		nowButton.addEventListener('click', () => {
			const liveNow = Math.floor(Date.now() / 1000);
			slider.value = String(clamp(liveNow));
			render();
		});

		slider.addEventListener('input', render);
		render();
	}

	function startNotifierTest() {
		const buttons = document.querySelectorAll('.notifier-test');
		if (!buttons.length) {
			return;
		}

		buttons.forEach((button) => {
			const eventPath = button.getAttribute('data-event');
			const puzzlePath = button.getAttribute('data-puzzle');
			if (!eventPath || !puzzlePath) {
				return;
			}

			const row = button.closest('tr');
			const status = row ? row.querySelector('.notifier-test-status') : null;

			button.addEventListener('click', async function () {
				button.disabled = true;
				if (status) {
					status.textContent = 'Sending…';
					status.className = 'notifier-test-status';
				}

				try {
					const response = await fetch(`/${encodeURIComponent(eventPath)}/admin/notifier/test/${encodeURIComponent(puzzlePath)}`, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
						},
					});
					const data = await response.json().catch(() => ({}));
					if (response.ok && data.ok) {
						if (status) {
							status.textContent = data.message || 'Sent.';
							status.className = 'notifier-test-status success';
						}
					} else {
						if (status) {
							status.textContent = data.error || `Request failed (${response.status})`;
							status.className = 'notifier-test-status error';
						}
					}
				} catch (err) {
					if (status) {
						status.textContent = 'Network error';
						status.className = 'notifier-test-status error';
					}
				} finally {
					button.disabled = false;
				}
			});
		});
	}

	function startResetProgress() {
		const button = document.getElementById('user-reset-progress');
		if (!button) {
			return;
		}

		const userId = button.getAttribute('data-user');
		if (!userId) {
			return;
		}

		const initialText = button.getAttribute('data-initial-text') || button.textContent;
		const clicksRequired = Math.max(1, Number(button.getAttribute('data-clicks-required')) || 4);
		const status = document.getElementById('user-reset-progress-status');

		let clicks = 0;
		let resetTimer = null;

		const setStatus = (text, cls) => {
			if (!status) return;
			status.textContent = text || '';
			status.className = cls || '';
		};

		const reset = () => {
			clicks = 0;
			button.textContent = initialText;
			if (resetTimer !== null) {
				clearTimeout(resetTimer);
				resetTimer = null;
			}
		};

		const armAutoReset = () => {
			if (resetTimer !== null) {
				clearTimeout(resetTimer);
			}
			resetTimer = setTimeout(reset, 5000);
		};

		button.addEventListener('click', async function () {
			if (button.disabled) {
				return;
			}

			clicks += 1;
			const remaining = clicksRequired - clicks;
			if (remaining > 0) {
				button.textContent = `Click ${remaining} more time${remaining === 1 ? '' : 's'} to reset progress`;
				setStatus('', '');
				armAutoReset();
				return;
			}

			if (resetTimer !== null) {
				clearTimeout(resetTimer);
				resetTimer = null;
			}

			button.disabled = true;
			button.textContent = 'Resetting…';
			setStatus('', '');

			try {
				const response = await fetch(`/api/user/${encodeURIComponent(userId)}/reset-progress`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: '{}',
				});
				const data = await response.json().catch(() => ({}));
				if (response.ok && data.ok) {
					button.textContent = 'Progress reset';
					setStatus('Reload the page to see the updated progress.', 'success');
				} else {
					setStatus(data.error || `Request failed (${response.status})`, 'error');
					reset();
					button.disabled = false;
				}
			} catch (err) {
				setStatus('Network error', 'error');
				reset();
				button.disabled = false;
			}
		});
	}

	function start() {
		const checkboxes = document.querySelectorAll('.user-checkbox');
		const errorSpan = document.getElementById('user-error');

		checkboxes.forEach((checkbox) => {
			let originalState = checkbox.checked;
			checkbox.addEventListener('pointerdown', function () {
				originalState = checkbox.checked;
			});
			checkbox.addEventListener('click', async function () {
				const userId = checkbox.getAttribute('data-user');
				const value = checkbox.getAttribute('data-value');
				const status = checkbox.checked;
				checkbox.disabled = true;
				if (errorSpan) errorSpan.textContent = '';

				try {
					const response = await fetch(`/api/user/${userId}`, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
						},
						body: JSON.stringify({ [value]: status }),
					});
					const data = await response.json();
					if (response.ok && data.user) {
						checkbox.checked = !!data.user[value];
						checkbox.disabled = false;
					} else {
						if (errorSpan && data.error) {
							errorSpan.textContent = data.error;
						}
						checkbox.checked = originalState;
						checkbox.disabled = false;
					}
				} catch (err) {
					if (errorSpan) {
						errorSpan.textContent = 'Network error';
					}
					checkbox.checked = originalState;
					checkbox.disabled = false;
				}
			});
			checkbox.disabled = false;
		});

		startLeaderboardChartControls();
		startNotifierTest();
		startResetProgress();
	};

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', start);
	} else {
		start();
	}
})();
