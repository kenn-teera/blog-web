// Runs in <head> so the saved theme applies before the page paints
(function () {
    try {
        const savedTheme = localStorage.getItem('theme');
        if (savedTheme) {
            document.documentElement.setAttribute('data-theme', savedTheme);
            return;
        }
    } catch (e) {
        // Storage unavailable; fall back to the system preference
    }
    if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        document.documentElement.setAttribute('data-theme', 'dark');
    }
})();
