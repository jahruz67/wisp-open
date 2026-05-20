// Main JavaScript for Settings UI

// Global state
let currentSettings = {};
let apiKeyVisible = true;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    await loadSettings();
    await loadMicrophones();

    // Show form after loading
    document.getElementById('loadingIndicator').style.display = 'none';
    document.getElementById('settingsForm').style.display = 'block';
});

// Load settings from backend
async function loadSettings() {
    try {
        const settings = await window.go.main.App.GetSettings();
        currentSettings = settings;

        console.log("Loaded settings:", settings);

        // Populate fields
        document.getElementById('apiKey').value = settings.api_key || '';
        document.getElementById('whisperModel').value = settings.whisper_model || 'whisper-large-v3-turbo';
        document.getElementById('aiModel').value = settings.ai_model || 'llama-3.3-70b-versatile';
        document.getElementById('aiPrompt').value = settings.ai_prompt || '';
        document.getElementById('startupToggle').checked = settings.startup || false;

        if (settings.linux_press_mode) {
            const section = document.getElementById('linuxPressSection');
            const cmdInput = document.getElementById('linuxPressCommand');
            if (section) section.style.display = 'block';
            if (cmdInput) cmdInput.value = settings.linux_press_command || '';
        }

        // Load history
        renderHistory(settings.history || []);

    } catch (err) {
        console.error("Failed to load settings:", err);
        alert("Failed to load settings: " + err);
    }
}

// Load microphones
async function loadMicrophones() {
    try {
        const mics = await window.go.main.App.GetMicrophones();
        const select = document.getElementById('micDevice');

        // Keep default option
        select.innerHTML = '<option value="default">System Default</option>';

        mics.forEach(mic => {
            if (mic.index !== -1) {
                const option = document.createElement('option');
                option.value = mic.index;
                option.textContent = mic.name;
                select.appendChild(option);
            }
        });

        // Set current selection
        if (currentSettings.microphone_device !== null && currentSettings.microphone_device !== undefined) {
            select.value = currentSettings.microphone_device;
        } else {
            select.value = "default";
        }

    } catch (err) {
        console.error("Failed to load microphones:", err);
    }
}

// Render history list
function renderHistory(history) {
    const container = document.getElementById('historyList');
    container.innerHTML = '';

    if (!history || history.length === 0) {
        container.innerHTML = '<div style="text-align: center; color: var(--text-muted); padding: 20px;">No history yet. Use your shortcut to start recording!</div>';
        return;
    }

    // Show newest first
    const sorted = [...history].reverse();

    sorted.forEach(item => {
        const el = document.createElement('div');
        el.className = 'history-item';

        const time = new Date(item.timestamp).toLocaleString();

        el.innerHTML = `
            <div class="history-time">${time}</div>
            <div style="white-space: pre-wrap;">${escapeHtml(item.text)}</div>
        `;
        container.appendChild(el);
    });
}

// Toggle API key visibility
function toggleApiKeyVisibility() {
    const input = document.getElementById('apiKey');
    const btn = document.getElementById('toggleKeyBtn');

    if (apiKeyVisible) {
        input.type = 'password';
        btn.textContent = 'Show';
        apiKeyVisible = false;
    } else {
        input.type = 'text';
        btn.textContent = 'Hide';
        apiKeyVisible = true;
    }
}

// Save API Key
async function saveApiKey() {
    const key = document.getElementById('apiKey').value.trim();
    if (!key) {
        alert('Please enter an API key');
        return;
    }
    await saveSetting('api_key', key);
    showToast('API Key saved');
}


async function copyLinuxPressCommand() {
    const cmdInput = document.getElementById('linuxPressCommand');
    if (!cmdInput) return;

    const command = cmdInput.value || '';
    try {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(command);
        } else {
            cmdInput.focus();
            cmdInput.select();
            document.execCommand('copy');
        }
        showToast('Command copied');
    } catch (err) {
        console.error('Failed to copy Linux command:', err);
    }
}

window.copyLinuxPressCommand = copyLinuxPressCommand;

// Save Whisper Model
async function saveWhisperModel() {
    const model = document.getElementById('whisperModel').value;
    await saveSetting('whisper_model', model);
    showToast('Whisper model saved');
}

// Save AI Model
async function saveAiModel() {
    const model = document.getElementById('aiModel').value;
    await saveSetting('ai_model', model);
    showToast('AI model saved');
}

// Save Microphone
async function saveMicDevice() {
    const val = document.getElementById('micDevice').value;
    let device = null;
    if (val !== "default") {
        device = parseInt(val);
    }
    await saveSetting('microphone_device', device);
    showToast('Microphone saved');
}

// Save Prompt
async function savePrompt() {
    const prompt = document.getElementById('aiPrompt').value.trim();
    await saveSetting('ai_prompt', prompt);
    showToast('Prompt saved');
}

// Toggle Startup
async function toggleStartup() {
    const enabled = document.getElementById('startupToggle').checked;
    try {
        const result = await window.go.main.App.ToggleStartup(enabled);
        if (result !== "Success") {
            alert("Failed to update startup: " + result);
            document.getElementById('startupToggle').checked = !enabled;
        }
    } catch (err) {
        console.error("Failed to toggle startup:", err);
    }
}

// Clear History
async function clearHistory() {
    if (confirm("Are you sure you want to clear all history?")) {
        await window.go.main.App.ClearHistory();
        renderHistory([]);
    }
}

// Helper to save a single setting
async function saveSetting(key, value) {
    try {
        const settings = {};
        settings[key] = value;
        const result = await window.go.main.App.SaveSettings(settings);
        currentSettings[key] = value;
        return result;
    } catch (err) {
        console.error(`Failed to save ${key}:`, err);
        alert(`Failed to save setting: ${err}`);
    }
}

// Helper to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Toast notification
function showToast(msg) {
    console.log(msg);

    // Create toast element
    let toast = document.getElementById('toast');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast';
        toast.style.cssText = `
            position: fixed;
            bottom: 20px;
            left: 50%;
            transform: translateX(-50%);
            background: var(--primary);
            color: white;
            padding: 12px 24px;
            border-radius: 8px;
            font-weight: 500;
            z-index: 1000;
            opacity: 0;
            transition: opacity 0.3s;
        `;
        document.body.appendChild(toast);
    }

    toast.textContent = msg;
    toast.style.opacity = '1';

    setTimeout(() => {
        toast.style.opacity = '0';
    }, 2000);
}
