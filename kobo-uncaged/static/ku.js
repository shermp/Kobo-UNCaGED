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

var kuConfig, kuAuth, libInfo, msgEvtSrc;

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
    msgEvtSrc.addEventListener('libInfo', function(ev) {
        getKUJson(kuInfo.libInfoPath, showLibraryInfo);
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
    var connAddBtn = document.getElementById('cfgAddConn');
    if (connAddBtn.dataset.eventConnAdd === "false") {
        connAddBtn.addEventListener('click', showAddConnection);
        connAddBtn.dataset.eventConnAdd = "true";
    }
    var connDelBtn = document.getElementById('cfgDelConn');
    if (connDelBtn.dataset.eventConnDelete === "false") {
        connDelBtn.addEventListener('click', deleteConnection);
        connDelBtn.dataset.eventConnDelete = "true";
    }
    var connOkBtn = document.getElementById('connOkBtn');
    if (connOkBtn.dataset.eventConnOk === 'false') {
        connOkBtn.addEventListener('click', addConnection);
        connOkBtn.dataset.eventConnOk = 'true';
    }
    var connCancelBtn = document.getElementById('connCancelBtn');
    if (connCancelBtn.dataset.eventConnCancel === 'false') {
        connCancelBtn.addEventListener('click', cancelAddConnection);
        connCancelBtn.dataset.eventConnCancel = 'true';
    }
    var directConnSel = document.getElementById('directConn');
    if (directConnSel.dataset.eventConnSel === 'false') {
        directConnSel.addEventListener('change', setConnDelState);
        directConnSel.dataset.eventConnSel = 'true';
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

function displayButtonState(btnID, pressed) {
    var btn = document.getElementById(btnID);
    if (pressed === true) {
        btn.style.backgroundColor = "#000000";
        btn.style.color = "#FFFFFF";
    } else {
        btn.style.backgroundColor = "#FFFFFF";
        btn.style.color = "#000000";
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
function showAddConnection(ev) {
    hideAllComponents();
    document.getElementById('kudirectconn').style.display = 'block';
}
function addConnection(ev) {
    var connName = document.getElementById('connName');
    var connHost = document.getElementById('connHost');
    var connPort = document.getElementById('connPort');
    var connMissing = document.getElementById('ku-conn-missing');
    if (connName.value.length === 0 || connHost.value.length === 0 || connPort.value.length === 0) {
        connMissing.style.display = 'block';
        return;
    }
    connMissing.style.display = 'none';
    var conn = {name: connName.value, host: connHost.value, port: parseInt(connPort.value)};
    kuConfig.opts.directConn.push(conn);

    var dc = document.getElementById('directConn');
    var connOpt = document.createElement('option');
    connOpt.text = conn.name;
    dc.add(connOpt);
    dc.selectedIndex = dc.length - 1;
    document.getElementById('cfgDelConn').disabled = false;
    hideAllComponents();
    document.getElementById('kuconfig').style.display = 'block';
}
function cancelAddConnection(ev) {
    hideAllComponents();
    document.getElementById('kuconfig').style.display = 'block';
}
function deleteConnection(ev) {
    var dc = document.getElementById('directConn');
    if (dc.selectedIndex > 0) {
        kuConfig.opts.directConn.splice(dc.selectedIndex - 1, 1);
        var newIndex = dc.selectedIndex - 1;
        dc.remove(dc.selectedIndex);
        dc.selectedIndex = newIndex;
    }
}
function setConnDelState(ev) {
    if (document.getElementById('directConn').selectedIndex > 0) {
        document.getElementById('cfgDelConn').disabled = false;
    } else {
        document.getElementById('cfgDelConn').disabled = true;
    }
}
function sendAuth() {
    displayButtonState('authLoginBtn', true)
    kuAuth.password = document.getElementById('password').value;
    var xhr = new XMLHttpRequest();
    xhr.open('POST', kuInfo.authPath);
    xhr.onload = function () {
        if (xhr.status === 204) {
            displayButtonState('authLoginBtn', false)
            hideAllComponents();
        } else {
            console.log('SendAuth status code expected was 204, got ' + xhr.status);
        }
    }
    xhr.send(JSON.stringify(kuAuth));
}
function showCalInstances(resp) {
    if (resp.status === 200) {
        var kuCalInstances = JSON.parse(resp.responseText);
        var l = document.getElementById('calInstanceList');
        l.innerHTML = '';
        for (var i = 0; i < kuCalInstances.length; i++) {
            var instListItem = document.createElement('li');
            instListItem.dataset.instanceHost = kuCalInstances[i].host;
            instListItem.dataset.instancePort = kuCalInstances[i].port;
            instListItem.dataset.instanceName = kuCalInstances[i].name;
            instListItem.innerHTML = kuCalInstances[i].host + ' :: ' + kuCalInstances[i].name;
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
            host: t.dataset.instanceHost,
            port: parseInt(t.dataset.instancePort, 10),
            name: t.dataset.instanceName,
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

function showLibraryInfo(resp) {
    if (resp.status === 200) {
        libInfo = JSON.parse(resp.responseText);
        var fieldSel = document.getElementById('kuSubtitleColumn');
        for (var i = 0; i < libInfo.subtitleFields.length; i++) {
            var fieldOpt = document.createElement('option');
            fieldOpt.value = libInfo.subtitleFields[i];
            fieldOpt.innerHTML = libInfo.subtitleFields[i];
            if (libInfo.currSel === i) {
                fieldOpt.selected = true;
            }
            fieldSel.appendChild(fieldOpt);
        }
        fieldSel.addEventListener('change', sendLibraryInfo);
        fieldSel.disabled = false;
    }
}

function sendLibraryInfo(ev) {
    var el = ev.target;
    if (el.id === 'kuSubtitleColumn') {
        libInfo.currSel = 0;
        if (el.selectedIndex > 0) {
            libInfo.currSel = el.selectedIndex;
        }
    }
    var xhr = new XMLHttpRequest();
    xhr.open('POST', kuInfo.libInfoPath);
    xhr.onload = function () {
        if (xhr.status !== 204) {
            console.log('showLibraryInfo status code expected was 204, got ' + xhr.status);
        }
    }
    xhr.send(JSON.stringify(libInfo));
}

function sendConfig() {
    displayButtonState('cfgExitBtn', true);
    var gl = document.getElementById('generateLevel');
    var rs = document.getElementById('resizeAlgorithm');
    kuConfig.opts.preferSDCard = document.getElementById('preferSDCard').checked;
    kuConfig.opts.preferKepub = document.getElementById('preferKepub').checked;
    kuConfig.opts.enableDebug = document.getElementById('enableDebug').checked;
    var exclFormats = document.getElementById('excludeFormats').value.split(",").map(function(item){return item.trim()});
    kuConfig.opts.excludeFormats = exclFormats.filter(function(e){return e;});
    kuConfig.opts.thumbnail.generateLevel = gl.options[gl.selectedIndex].value;
    kuConfig.opts.thumbnail.resizeAlgorithm = rs.options[rs.selectedIndex].value;
    var jpgQuality = parseInt(document.getElementById('jpegQuality').value);
    if (jpgQuality < 50) {
        jpgQuality = 50;
    }
    kuConfig.opts.thumbnail.jpegQuality = jpgQuality;
    kuConfig.opts.directConnIndex = document.getElementById('directConn').selectedIndex - 1;
    var xhr = new XMLHttpRequest();
    xhr.open('POST', kuInfo.configPath);
    xhr.onload = function (btn) {
        if (xhr.status === 204) {
            displayButtonState('cfgExitBtn', false);
            hideAllComponents();
        } else {
            console.log('sendConfig: status code expected was 204, got ' + xhr.status);
        }
    };
    xhr.send(JSON.stringify(kuConfig));
}
function exitKU() {
    displayButtonState('cfgExitBtn', true)
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
    displayButtonState('cfgDisconnectBtn', true)
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
        document.getElementById('excludeFormats').value = kuConfig.opts.excludeFormats.toString();
        document.getElementById('generateLevel').value = kuConfig.opts.thumbnail.generateLevel;
        document.getElementById('resizeAlgorithm').value = kuConfig.opts.thumbnail.resizeAlgorithm;
        document.getElementById('jpegQuality').value = kuConfig.opts.thumbnail.jpegQuality;
        var dc = document.getElementById('directConn');
        if (kuConfig.opts.directConnIndex < 0) {
            dc.selectedIndex = 0;
        }
        for(var i = 0; i < kuConfig.opts.directConn.length; i++) {
            var connOpt = document.createElement('option');
            connOpt.text = kuConfig.opts.directConn[i].name;
            dc.add(connOpt);
            if (i === kuConfig.opts.directConnIndex) {
                dc.selectedIndex = i + 1;
            }
        }
        if (dc.selectedIndex == 0) {
            document.getElementById('cfgDelConn').disabled = true;
        }
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
