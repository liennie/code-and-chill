(function () {
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
	};

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
