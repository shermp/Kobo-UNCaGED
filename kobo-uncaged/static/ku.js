// Adapted from https://github.com/tombigel/detect-zoom
function getKoboZoom() {
    var deviceWidth = (Math.abs(window.orientation) == 90) ? screen.height : screen.width;
    var zoom = deviceWidth / window.innerWidth;
    return zoom;
}

// The Kobo browser doesn't take DPI into account, so at min zoom level, text is tiny
function setKoboSizes(dpi) {
    var height = Math.round(window.innerHeight * 0.95);
    var width = Math.round(window.innerWidth * 0.95);
    var baseFontSize = 16 * (dpi / 96);
    baseFontSize = (baseFontSize * 0.8) / getKoboZoom();
    document.documentElement.style.fontSize = baseFontSize.toFixed(4) + 'px';
    var containerDiv = document.getElementById("kuapp");
    containerDiv.style.height = height + 'px';
    containerDiv.style.width = width + 'px';
    containerDiv.style.margin = 'auto';
}

// Abstract away the AJAX GET boilerplate
function getKUJson(path, responseHandler) {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', path);
    xhr.onload = function() {
        responseHandler(xhr);
    };
    xhr.send();
}
function hideAllComponents() {
    var c = document.getElementById('kuapp').children;
    for (var i = 0; i < c.length; i++) {
        c[i].style.display = 'none';
    } 
}

var kuConfig, kuAuth, msgEvtSrc;

function setupSSE() {
    msgEvtSrc = new EventSource(kuInfo.ssePath);
    msgEvtSrc.addEventListener('showMessage', showMessage);
    msgEvtSrc.addEventListener('progress', showProgress);
    msgEvtSrc.addEventListener('auth', function(ev) {
        getKUJson(kuInfo.authPath, showAuthDlg);
    });
    msgEvtSrc.addEventListener('calibreInstances', function (ev) {
        getKUJson(kuInfo.instancePath, showCalInstances);
    });
    msgEvtSrc.addEventListener('kuFinished', showFinishedMsg);
}
function setupEventHandlers() {
    var startBtn = document.getElementById('cfgStartBtn');
    if (startBtn.dataset.eventStart === "false") {
        startBtn.addEventListener('click', sendConfig);
        startBtn.dataset.eventStart = "true";
    }
    var exitBtn = document.getElementById('cfgExitBtn');
    if (exitBtn.dataset.eventExit === "false") {
        exitBtn.addEventListener('click', exitKU);
        exitBtn.dataset.eventExit = "true";
    }
    var disconnectBtn = document.getElementById('cfgDisconnectBtn');
    if (disconnectBtn.dataset.eventDisconnect === "false") {
        disconnectBtn.addEventListener('click', disconnectKU);
        disconnectBtn.dataset.eventDisconnect = "true";
    }
    var authLoginBtn = document.getElementById('authLoginBtn');
    if (authLoginBtn.dataset.eventAuthLogin === "false") {
        authLoginBtn.addEventListener('click', sendAuth);
        authLoginBtn.dataset.eventAuthLogin = "true";
    }
    var pwInput = document.getElementById('password');
    if (pwInput.dataset.eventAuthLoginEnt === "false") {
        pwInput.addEventListener('keypress', function (e){
            if (e.key === 'Enter') {
                e.preventDefault();
                authLoginBtn.click();
            }
        });
        pwInput.dataset.eventAuthLoginEnt = "true";
    }
    var instList = document.getElementById('calInstanceList');
    if (instList.dataset.eventInstances === "false") {
        instList.addEventListener('click', selectCalInstance);
        instList.dataset.eventInstances = "true";
    }
    var cfgLabels = document.querySelectorAll(".ku-cfg-row > label");
    for (var i = 0; i < cfgLabels.length; i++) {
        cfgLabels[i].addEventListener('click', showCfgHelpText);
    }
}

function showMessage(ev) {
    var msgDiv = document.getElementById('kumessage');
    if (msgDiv.style.display !== 'block') {
        hideAllComponents();
        msgDiv.style.display = 'block';
    } 
    document.getElementById('ku-msgbox').innerHTML = ev.data;
}
function showProgress(ev) {
    var prog = document.getElementById('ku-progress');
    if (ev.data >= 0 && ev.data <= 100) {
        var msgDiv = document.getElementById('kumessage');
        if (msgDiv.style.display !== 'block') {
            hideAllComponents();
            msgDiv.style.display = 'block';
        }
        prog.value = ev.data;
        prog.style.visibility = 'visible';
    } else {
        prog.style.visibility = 'hidden';
    }
}
function showAuthDlg(resp) {
    if (resp.status === 200) {
        kuAuth = JSON.parse(resp.responseText);
        var authDiv = document.getElementById('kuauth');
        if (authDiv.style.display !== 'block') {
            hideAllComponents();
            document.getElementById('authLibName').innerHTML = kuAuth.libName;
            authDiv.style.display = 'block';
        }
        document.getElementById('password').value = '';
    }
}
function sendAuth() {
    kuAuth.password = document.getElementById('password').value;
    var xhr = new XMLHttpRequest();
    xhr.open('POST', kuInfo.authPath);
    xhr.onload = function () {
        if (xhr.status === 204) {
            hideAllComponents();
        } else {
            console.log('SendAuth status code expected was 204, got ' + xhr.status);
        }
    }
    xhr.send(JSON.stringify(kuAuth));
}
function showCalInstances(resp) {
    if (resp.status === 200) {
        kuCalInstances = JSON.parse(resp.responseText);
        var l = document.getElementById('calInstanceList');
        l.innerHTML = '';
        for (var i = 0; i < kuCalInstances.length; i++) {
            var instListItem = document.createElement('li');
            instListItem.dataset.instanceAddr = kuCalInstances[i].Addr;
            instListItem.dataset.instanceDescription = kuCalInstances[i].Description;
            instListItem.innerHTML = kuCalInstances[i].Addr + ' :: ' + kuCalInstances[i].Description;
            l.appendChild(instListItem);
        }
        var instDiv = document.getElementById('kuinstances');
        if (instDiv.style.display !== "block") {
            hideAllComponents();
            instDiv.style.display = "block";
        }
    }
}
function selectCalInstance(ev) {
    if (ev.target && ev.target.nodeName === 'LI') {
        var t = ev.target;
        var calInstance = {
            Addr: t.dataset.instanceAddr,
            Description: t.dataset.instanceDescription,
        }
        var xhr = new XMLHttpRequest();
        xhr.open('POST', kuInfo.instancePath);
        xhr.onload = function () {
            if (xhr.status === 204) {
                hideAllComponents();
            } else {
                console.log('calInstance status code expected was 204, got ' + xhr.status);
            }
        }
        xhr.send(JSON.stringify(calInstance));
    }
}

function sendConfig() {
    var gl = document.getElementById('generateLevel');
    var rs = document.getElementById('resizeAlgorithm');
    kuConfig.opts.preferSDCard = document.getElementById('preferSDCard').checked;
    kuConfig.opts.preferKepub = document.getElementById('preferKepub').checked;
    kuConfig.opts.enableDebug = document.getElementById('enableDebug').checked;
    kuConfig.opts.thumbnail.generateLevel = gl.options[gl.selectedIndex].value;
    kuConfig.opts.thumbnail.resizeAlgorithm = rs.options[rs.selectedIndex].value;
    kuConfig.opts.thumbnail.jpegQuality = parseInt(document.getElementById('jpegQuality').value);
    var xhr = new XMLHttpRequest();
    xhr.open('POST', kuInfo.configPath);
    xhr.onload = function () {
        if (xhr.status === 204) {
            hideAllComponents();
        } else {
            console.log('sendConfig: status code expected was 204, got ' + xhr.status);
        }
    };
    xhr.send(JSON.stringify(kuConfig));
}
function exitKU() {
    getKUJson(kuInfo.exitPath, function(resp) {
        if (resp.status === 204) {
            hideAllComponents();
            var exitDiv = document.getElementById('kuexit');
            exitDiv.innerHTML = '<h2>Goodbye!</h2>';
            exitDiv.style.display = 'block';
        }
    });
}
function showFinishedMsg(ev) {
    hideAllComponents();
    var exitDiv = document.getElementById('kuexit');
    exitDiv.innerHTML = '<h2>' + ev.data + '</h2>';
    exitDiv.style.display = 'block';
}
function disconnectKU() {
    getKUJson(kuInfo.disconnectPath, function(resp) {
        if (resp.status === 204) {
            document.getElementById('cfgDisconnectBtn').style.display = 'none';
        }
    });
}
function handleShowKUCfg(resp) {
    if (resp.status === 200) {
        hideAllComponents();
        kuConfig = JSON.parse(resp.responseText);
        document.getElementById('preferSDCard').checked = kuConfig.opts.preferSDCard;
        document.getElementById('preferKepub').checked = kuConfig.opts.preferKepub;
        document.getElementById('enableDebug').checked = kuConfig.opts.enableDebug;
        document.getElementById('generateLevel').value = kuConfig.opts.thumbnail.generateLevel;
        document.getElementById('resizeAlgorithm').value = kuConfig.opts.thumbnail.resizeAlgorithm;
        document.getElementById('jpegQuality').value = kuConfig.opts.thumbnail.jpegQuality;
        document.getElementById('kuconfig').style.display = 'block';
    }
}
function showCfgHelpText(ev) {
    var lbl = ev.target;
    if ("helpText" in lbl.dataset) {
        var help = document.getElementById("cfgHelp");
        help.innerHTML = "<b>" + lbl.innerText + "</b><br/>" + lbl.dataset.helpText;
    }
}

window.onload = function() {
    setKoboSizes(kuInfo.screenDPI);
    setupEventHandlers();
    setupSSE();
    getKUJson(kuInfo.configPath, handleShowKUCfg);
};
window.onresize = function() {
    setKoboSizes(kuInfo.screenDPI);
};
