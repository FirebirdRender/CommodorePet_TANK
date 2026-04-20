// State
let gameState = 'lobby';
let roomCode = '';
let playerID = 0;
let token = '';
let playerName = '';
let difficulty = 5;
let vsAI = false;
let eventSource = null;
let pollInterval = null;

document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    handleDeepLink();
    showLobby();
});

function setupEventListeners() {
    document.getElementById('btn-create').addEventListener('click', createRoom);
    document.getElementById('btn-ai').addEventListener('click', () => { vsAI = true; createRoom(); });
    document.getElementById('btn-join').addEventListener('click', joinRoom);
    document.getElementById('btn-cancel').addEventListener('click', cancelRoom);
    document.getElementById('copy-btn').addEventListener('click', copyRoomCode);
    
    const diffSlider = document.getElementById('difficulty');
    const diffValue = document.getElementById('difficulty-value');
    diffSlider.addEventListener('input', (e) => {
        difficulty = parseInt(e.target.value);
        diffValue.textContent = difficulty;
    });

    document.getElementById('player-name').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') createRoom();
    });

    document.getElementById('join-code').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') joinRoom();
    });

    document.getElementById('join-code').addEventListener('input', (e) => {
        e.target.value = e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '');
    });

    document.getElementById('btn-back-lobby').addEventListener('click', showLobby);
    document.getElementById('btn-leave-spectate').addEventListener('click', () => {
        cleanupGlobalSSE();
        showLobby();
    });
    document.getElementById('btn-spectate')?.addEventListener('click', showMatchList);
}

function handleDeepLink() {
    const params = new URLSearchParams(window.location.search);
    const room = params.get('room');
    if (room) {
        document.getElementById('join-code').value = room.toUpperCase();
        joinRoom();
    }
}

async function createRoom() {
    const nameInput = document.getElementById('player-name');
    const name = nameInput.value.trim();
    if (!name) { showError('Enter your name'); return; }

    try {
        const res = await fetch('/api/room', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ difficulty: difficulty, player_name: name, vs_ai: vsAI })
        });

        if (!res.ok) {
            const err = await res.json();
            showError(err.message || 'Failed to create room');
            vsAI = false;
            return;
        }

        const data = await res.json();
        roomCode = data.room_code;
        playerID = data.player_id;
        token = data.token;
        playerName = data.player_name;

        showWaitingScreen();
        startEventStream();
        vsAI = false;
    } catch (e) {
        showError('Network error: ' + e.message);
        vsAI = false;
    }
}

async function joinRoom() {
    const nameInput = document.getElementById('player-name');
    const name = nameInput.value.trim();
    if (!name) { showError('Enter your name'); return; }
    
    const codeInput = document.getElementById('join-code');
    const code = codeInput.value.trim().toUpperCase();
    if (code.length !== 4) { showError('Room code must be 4 characters'); return; }
    
    try {
        const res = await fetch(`/api/room/${code}/join`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ player_name: name })
        });
        
        if (!res.ok) {
            const err = await res.json();
            showError(err.message || 'Failed to join room');
            return;
        }
        
        const data = await res.json();
        roomCode = data.room_code;
        playerID = data.player_id;
        token = data.token;
        playerName = data.player_name;
        
        cleanup();
        redirectToGame();
    } catch (e) {
        showError('Network error: ' + e.message);
    }
}

function startEventStream() {
    if (eventSource) eventSource.close();
    
    eventSource = new EventSource(`/api/room/${roomCode}/events`);
    
    eventSource.addEventListener('player_joined', (e) => {
        const data = JSON.parse(e.data);
        document.getElementById('waiting-status').textContent = 'VS ' + data.opponent_name;
        showWaitingScreen();
        setTimeout(() => {
            cleanup();
            redirectToGame();
        }, 500);
    });
    
    eventSource.addEventListener('game_starting', (e) => {
        cleanup();
        redirectToGame();
    });
    
    eventSource.addEventListener('room_expired', (e) => {
        cleanup();
        showError('Room expired');
        showLobby();
    });

    eventSource.addEventListener('bot_joined', (e) => {
        document.getElementById('waiting-status').textContent = 'VS CPU';
        setTimeout(() => {
            cleanup();
            redirectToGame();
        }, 500);
    });

    eventSource.addEventListener('auto_fill_started', (e) => {
        document.getElementById('waiting-status').textContent = 'AUTO-FILL: FINDING OPPONENT...';
    });

    eventSource.onerror = () => {
        if (eventSource) eventSource.close();
        startPolling();
    };
}

function startPolling() {
    if (pollInterval) return;
    pollInterval = setInterval(async () => {
        try {
            const res = await fetch(`/api/room/${roomCode}/status`);
            if (!res.ok) {
                showError('Room not found');
                cleanup();
                showLobby();
                return;
            }
            const data = await res.json();
            if (data.status === 'playing' || data.status === 'game_over') {
                cleanup();
                redirectToGame();
            } else if (data.status === 'closed' || data.status === 'expired') {
                cleanup();
                showError('Room closed');
                showLobby();
            }
        } catch (e) { }
    }, 2000);
}

function cleanup() {
    if (eventSource) eventSource.close();
    if (pollInterval) clearInterval(pollInterval);
    eventSource = null;
    pollInterval = null;
}

function redirectToGame() {
    window.location.href = `/game.html?room=${roomCode}&pid=${playerID}&token=${encodeURIComponent(token)}&name=${encodeURIComponent(playerName)}`;
}

function copyRoomCode() {
    const btn = document.getElementById('copy-btn');
    const fallback = function(text) {
        // Fallback for non-HTTPS contexts: use temporary textarea
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        ta.style.top = '-9999px';
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        try {
            document.execCommand('copy');
            btn.textContent = 'COPIED!';
        } catch (e) {
            btn.textContent = 'COPY FAILED';
        }
        document.body.removeChild(ta);
        setTimeout(function() { btn.textContent = 'COPY'; }, 2000);
    };

    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(roomCode).then(function() {
            btn.textContent = 'COPIED!';
            setTimeout(function() { btn.textContent = 'COPY'; }, 2000);
        }).catch(function() {
            fallback(roomCode);
        });
    } else {
        fallback(roomCode);
    }
}

function cancelRoom() {
    cleanup();
    roomCode = '';
    playerID = 0;
    token = '';
    showLobby();
}

function showLobby() {
    hideAllScreens();
    document.getElementById('screen-lobby').classList.remove('hidden');
    document.getElementById('player-name').focus();
}

function showWaitingScreen() {
    hideAllScreens();
    document.getElementById('screen-waiting').classList.remove('hidden');
    document.getElementById('room-code-display').textContent = roomCode;
    document.getElementById('difficulty-display').textContent = 'DIFFICULTY: ' + difficulty;
    if (!document.getElementById('waiting-status').textContent.includes('VS')) {
        if (vsAI) {
            document.getElementById('waiting-status').textContent = 'WAITING FOR CPU OPPONENT...';
        } else {
            document.getElementById('waiting-status').textContent = 'WAITING FOR OPPONENT...';
        }
    }
}

function showJoinScreen(code) {
    hideAllScreens();
    document.getElementById('screen-join').classList.remove('hidden');
    document.getElementById('join-display-code').textContent = code;
}

function showError(msg) {
    const banner = document.getElementById('error-banner');
    banner.textContent = msg;
    banner.classList.remove('hidden');
    setTimeout(() => { banner.classList.add('hidden'); }, 5000);
}

function hideAllScreens() {
    document.querySelectorAll('.screen').forEach(el => el.classList.add('hidden'));
}

let globalEventSource = null;
let reconnectTimeouts = {};

function cleanupGlobalSSE() {
    if (globalEventSource) {
        globalEventSource.close();
        globalEventSource = null;
    }
    reconnectTimeouts = {};
}

function showMatchList() {
    hideAllScreens();
    document.getElementById('screen-matchlist').classList.remove('hidden');
    fetchPlayingRooms();
    startGlobalEventSource();
}

async function fetchPlayingRooms() {
    try {
        const res = await fetch('/api/rooms?filter=playing');
        if (!res.ok) return;
        const rooms = await res.json();
        renderMatchList(rooms);
    } catch (e) { }
}

function renderMatchList(rooms) {
    const container = document.getElementById('match-list-container');
    container.innerHTML = '';

    if (rooms.length === 0) {
        container.innerHTML = '<div class="status-text">NO ACTIVE MATCHES</div>';
        return;
    }

    rooms.forEach(room => {
        const card = document.createElement('div');
        card.className = 'match-card';

        const players = room.players || [];
        const p1 = players[0] || { name: '???' };
        const p2 = players[1] || { name: '???' };

        card.innerHTML =
            '<div class="match-info">' +
                '<div class="match-code">' + room.code + '</div>' +
                '<div class="match-diff">DIFF: ' + room.difficulty + '</div>' +
            '</div>' +
            '<div class="match-players">' +
                '<div>' + p1.name + '</div>' +
                '<div class="match-vs">VS</div>' +
                '<div>' + p2.name + '</div>' +
            '</div>' +
            '<button class="btn spectate-btn" data-code="' + room.code + '">WATCH</button>';

        if (room.state === 'waiting_reconnect') {
            const badge = document.createElement('div');
            badge.className = 'reconnect-badge';
            badge.textContent = 'RECONNECT';
            card.appendChild(badge);
        }

        container.appendChild(card);
    });

    container.querySelectorAll('.spectate-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const code = btn.getAttribute('data-code');
            window.location.href = '/spectate/' + code;
        });
    });
}

function startGlobalEventSource() {
    if (globalEventSource) globalEventSource.close();

    globalEventSource = new EventSource('/api/events');

    globalEventSource.addEventListener('match_available', () => {
        fetchPlayingRooms();
    });

    globalEventSource.addEventListener('match_started', (e) => {
        const data = JSON.parse(e.data);
        reconnectTimeouts[data.room_code] = setTimeout(() => {
            delete reconnectTimeouts[data.room_code];
        }, 30000);
        fetchPlayingRooms();
    });

    globalEventSource.addEventListener('match_ended', (e) => {
        const data = JSON.parse(e.data);
        clearTimeout(reconnectTimeouts[data.room_code]);
        delete reconnectTimeouts[data.room_code];
        fetchPlayingRooms();
    });

    globalEventSource.addEventListener('player_reconnected', (e) => {
        const data = JSON.parse(e.data);
        clearTimeout(reconnectTimeouts[data.room_code]);
        delete reconnectTimeouts[data.room_code];
        fetchPlayingRooms();
    });

    globalEventSource.addEventListener('player_disconnected', (e) => {
        const data = JSON.parse(e.data);
        reconnectTimeouts[data.room_code] = setTimeout(() => {
            delete reconnectTimeouts[data.room_code];
        }, 30000);
        fetchPlayingRooms();
    });

    globalEventSource.onerror = () => {
        globalEventSource.close();
        globalEventSource = null;
    };
}
