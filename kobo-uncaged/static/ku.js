// Adapted from https://github.com/tombigel/detect-zoom
function getKoboZoom() {
    var deviceWidth = (Math.abs(window.orientation) == 90) ? screen.height : screen.width;
    var zoom = deviceWidth / window.innerWidth;
    return zoom;
}

// The Kobo browser doesn't take DPI into account, so at min zoom level, text is tiny
function setKoboSizes(dpi) {
    var height = Math.round(window.innerHeight * 0.95);
    var baseFontSize = 16 * (dpi / 96);
    baseFontSize = (baseFontSize * 0.8) / getKoboZoom();
    document.documentElement.style.fontSize = baseFontSize.toFixed(4) + 'px';
    var containerDiv = document.getElementById("ku_container");
    containerDiv.style.height = height + 'px';
}