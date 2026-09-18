(function () {
	function init() {
		var slides = Array.prototype.slice.call(document.querySelectorAll('.slide'));
		if (!slides.length) return;

		var index = 0;
		slides[index].classList.add('active');

		function show(i) {
			if (i < 0 || i >= slides.length) return;
			slides[index].classList.remove('active');
			index = i;
			slides[index].classList.add('active');
		}

		// Fullscreen must be requested from a user gesture, so this is only
		// ever called from within a keydown/click handler.
		function enterFullscreen() {
			var el = document.documentElement;
			if (!document.fullscreenElement && el.requestFullscreen) {
				el.requestFullscreen().catch(function () { /* no gesture, or denied */ });
			}
		}

		function leave() {
			var back = document.body.dataset.back;
			if (!back) return;
			if (document.fullscreenElement && document.exitFullscreen) {
				document.exitFullscreen().catch(function () { /* noop */ });
			}
			window.location.href = back;
		}

		var leaveLink = document.querySelector('.leave');
		if (leaveLink) {
			leaveLink.addEventListener('click', function (ev) {
				ev.preventDefault();
				leave();
			});
		}

		document.addEventListener('click', function (ev) {
			if (leaveLink && (ev.target === leaveLink || leaveLink.contains(ev.target))) return;
			enterFullscreen();
		});

		document.addEventListener('keydown', function (ev) {
			switch (ev.key) {
				case 'ArrowRight':
				case 'ArrowDown':
				case ' ':
					enterFullscreen();
					show(index + 1);
					ev.preventDefault();
					break;

				case 'ArrowLeft':
				case 'ArrowUp':
					enterFullscreen();
					show(index - 1);
					ev.preventDefault();
					break;

				case 'Escape':
				case 'Esc':
					ev.preventDefault();
					leave();
					break;

				default:
					// Fall back to keyCode for older/quirky browsers that
					// don't report ev.key for Escape.
					if (ev.keyCode === 27) {
						ev.preventDefault();
						leave();
					}
			}
		}, true);
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', init);
	} else {
		init();
	}
})();
