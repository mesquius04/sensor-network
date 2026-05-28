// ================== CONFIGURACIÓN ==================
const API_BASE = 'http://localhost:3000/api';
const REFRESH_INTERVAL = 5000;

const dashboard = document.getElementById('dashboard');
const connectionStatus = document.getElementById('connection-status');

let lastSuccessfulData = null;
const FALLBACK_ROOMS = [
    "Living Room",
    "Kitchen",
    "Children's Bedroom",
    "Father's Bedroom",
    "Bathroom"
];

const pendingDesiredTemp = {};

// ================== MANEJO DE TEMA ================== si
const themeToggle = document.getElementById('theme-toggle');
const html = document.documentElement;
const icon = themeToggle.querySelector('i');

function setTheme(theme) {
    html.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
    icon.className = theme === 'dark' ? 'fas fa-sun' : 'fas fa-moon';
}

// Cargar tema guardado
const savedTheme = localStorage.getItem('theme') || 'light';
setTheme(savedTheme);

themeToggle.addEventListener('click', () => {
    const current = html.getAttribute('data-theme');
    setTheme(current === 'dark' ? 'light' : 'dark');
});

// ================== FUNCIONES PRINCIPALES ==================
async function fetchRoomData() {
    try {
        const response = await fetch(`${API_BASE}/sensores`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json();
        lastSuccessfulData = data;
        
        connectionStatus.textContent = '✅ Connected - Live data';
        connectionStatus.style.color = '#27ae60';
        updateDashboard(data);
    } catch (error) {
        console.error('Error fetching data:', error);
        connectionStatus.textContent = '❌ Unable to connect to the server';
        connectionStatus.style.color = '#e74c3c';
        
        if (lastSuccessfulData) {
            const offlineData = clearRoomData(lastSuccessfulData);
            updateDashboard(offlineData);
            connectionStatus.textContent += ' (showing cached rooms)';
        } else {
            const skeletonData = buildSkeletonData(FALLBACK_ROOMS);
            updateDashboard(skeletonData);
            connectionStatus.textContent += ' (rooms)';
        }
    }
}

function clearRoomData(originalData) {
    const cleaned = {};
    for (const [room, values] of Object.entries(originalData)) {
        cleaned[room] = {
            temp: undefined,
            desiredTemp: undefined,
            hum: undefined,
            sound: undefined,
            light: false
        };
    }
    return cleaned;
}

function buildSkeletonData(roomList) {
    const data = {};
    roomList.forEach(room => {
        data[room] = {
            temp: undefined,
            desiredTemp: undefined,
            hum: undefined,
            sound: undefined,
            light: false
        };
    });
    return data;
}

function updateDashboard(data) {
    dashboard.innerHTML = '';

    if (!data || Object.keys(data).length === 0) {
        dashboard.innerHTML = '<p style="text-align:center; grid-column:1/-1;">No rooms configured yet.</p>';
        return;
    }

    for (const [roomName, roomData] of Object.entries(data)) {
        const card = createCard(roomName, roomData);
        dashboard.appendChild(card);
    }
}

function createCard(name, data) {
    const div = document.createElement('div');
    div.className = 'card';
    // Borde dorado si la luz está encendida
    if (data.light) div.classList.add('light-on');

    let displayDesired = data.desiredTemp;
    let isPending = false;

    if (pendingDesiredTemp.hasOwnProperty(name)) {
        displayDesired = pendingDesiredTemp[name];
        isPending = true;
    }

    // Determinar clase de color según temperatura actual
    let tempClass = '';
    if (data.temp !== undefined) {
        if (data.temp < 18) tempClass = 'temp-cold';
        else if (data.temp > 24) tempClass = 'temp-hot';
        else tempClass = 'temp-warm';
    }

    div.innerHTML = `
        <h2>${name}</h2>
        <div class="sensor" style="border-bottom: none; padding-bottom: 0;">
            <span>🌡️ Current Temp</span>
            <strong class="${tempClass}">${data.temp?.toFixed(1) ?? '--'} °C</strong>
        </div>

        <div class="temp-controls">
            <span>🎯 Desired Temp</span>
            <div style="display: flex; align-items: center; gap: 10px;">
                <button class="btn-temp" data-room="${name}" data-change="-0.5">-</button>
                <span class="desired-val ${isPending ? 'pending' : ''}">${displayDesired?.toFixed(1) ?? '--'}°C</span>
                <button class="btn-temp" data-room="${name}" data-change="0.5">+</button>
                <button class="btn-set-temp ${isPending ? 'visible' : ''}" data-room="${name}">Set</button>
            </div>
        </div>

        <div class="sensor"><span>💧 Humidity</span><span>${data.hum ?? '--'} %</span></div>
        <div class="sensor"><span>🔊 Sound</span><span>${data.sound ?? '--'} dB</span></div>
        
        <span class="control-label">💡 Lighting Control</span>
        <div class="light-control">
            <button class="btn-light ${data.light ? 'on' : ''}" data-room="${name}">
                <i class="fas fa-power-off"></i> ${data.light ? 'ON' : 'OFF'}
            </button>
        </div>
    `;
    
    // Listeners de temperatura
    div.querySelectorAll('.btn-temp').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const room = e.target.dataset.room;
            const change = parseFloat(e.target.dataset.change);
            incrementLocalTemp(room, change);
        });
    });

    const setBtn = div.querySelector('.btn-set-temp');
    if (setBtn) {
        setBtn.addEventListener('click', () => {
            sendDesiredTemp(name);
        });
    }

    // Listener para el botón único de luz
    const lightBtn = div.querySelector('.btn-light');
    if (lightBtn) {
        lightBtn.addEventListener('click', () => {
            toggleLight(name);
        });
    }
    
    return div;
}

async function toggleLight(room) {
    // Aquí enviarías POST/PUT a /api/rooms/{room}/light
    // Por ahora simulamos el cambio y refrescamos
    alert(`💡 Toggling light in ${room}`);
    setTimeout(fetchRoomData, 200);
}

function incrementLocalTemp(room, change) {
    let currentVal = null;
    const card = findCardByRoom(room);
    if (card) {
        const valSpan = card.querySelector('.desired-val');
        if (valSpan) {
            currentVal = parseFloat(valSpan.textContent);
        }
    }
    
    if (isNaN(currentVal)) {
        if (lastSuccessfulData && lastSuccessfulData[room] && lastSuccessfulData[room].desiredTemp != null) {
            currentVal = lastSuccessfulData[room].desiredTemp;
        } else {
            currentVal = 20.0;
        }
    }

    let newTemp = currentVal + change;
    newTemp = Math.max(15, Math.min(30, Math.round(newTemp * 2) / 2));

    pendingDesiredTemp[room] = newTemp;

    if (card) {
        const valSpan = card.querySelector('.desired-val');
        if (valSpan) {
            valSpan.textContent = newTemp.toFixed(1) + '°C';
            valSpan.classList.add('pending');
        }
        const setBtn = card.querySelector('.btn-set-temp');
        if (setBtn) {
            setBtn.classList.add('visible');
        }
    }
}

async function sendDesiredTemp(room) {
    const newTemp = pendingDesiredTemp[room];
    if (newTemp === undefined) return;

    try {
        const response = await fetch(
            `${API_BASE}/rooms/${encodeURIComponent(room)}/desiredTemp`,
            {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ temp: newTemp })
            }
        );
        if (!response.ok) throw new Error('Failed to update desired temp');
        
        delete pendingDesiredTemp[room];
        const card = findCardByRoom(room);
        if (card) {
            const valSpan = card.querySelector('.desired-val');
            if (valSpan) {
                valSpan.classList.remove('pending');
            }
            const setBtn = card.querySelector('.btn-set-temp');
            if (setBtn) {
                setBtn.classList.remove('visible');
            }
        }
        fetchRoomData();
    } catch (error) {
        alert('Could not update desired temperature. Server might be offline.');
        console.error(error);
    }
}

function findCardByRoom(room) {
    const cards = document.querySelectorAll('.card');
    for (const card of cards) {
        const h2 = card.querySelector('h2');
        if (h2 && h2.textContent === room) {
            return card;
        }
    }
    return null;
}

document.addEventListener('DOMContentLoaded', () => {
    fetchRoomData();
    setInterval(fetchRoomData, REFRESH_INTERVAL);
});

document.getElementById('btn-talk').addEventListener('click', () => {
    alert('Listening...');
});