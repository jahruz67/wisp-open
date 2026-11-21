// Wisp (WIS Free V3) Website Interactive Controller

document.addEventListener('DOMContentLoaded', () => {
  initHeader();
  initDictationSimulator();
  initModelTabs();
  initDistroSwitcher();
  initCopyButtons();
  initScrollReveal();
  initCurrentYear();
});

/* 1. Header Navigation & Mobile Menu */
function initHeader() {
  const header = document.querySelector('.site-header');
  const toggle = document.querySelector('.nav-toggle');
  const navLinks = document.querySelector('.nav-links');

  if (header) {
    window.addEventListener('scroll', () => {
      header.classList.toggle('scrolled', window.scrollY > 15);
    }, { passive: true });
  }

  if (toggle && navLinks) {
    toggle.addEventListener('click', () => {
      const isOpen = navLinks.classList.toggle('open');
      toggle.setAttribute('aria-expanded', String(isOpen));
      toggle.setAttribute('aria-label', isOpen ? 'Close navigation' : 'Open navigation');
    });

    navLinks.querySelectorAll('a').forEach(link => {
      link.addEventListener('click', () => {
        navLinks.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
      });
    });

    document.addEventListener('click', (e) => {
      if (!header.contains(e.target) && navLinks.classList.contains('open')) {
        navLinks.classList.remove('open');
        toggle.setAttribute('aria-expanded', 'false');
      }
    });
  }
}

/* 2. Interactive Desktop Dictation & Floating HUD Overlay Simulator */
function initDictationSimulator() {
  const overlay = document.getElementById('hudOverlay');
  const hudIcon = document.getElementById('hudIcon');
  const hudTitle = document.getElementById('hudTitle');
  const hudSub = document.getElementById('hudSub');
  const hudTimer = document.getElementById('hudTimer');
  const waveBars = document.querySelectorAll('.hud-waveform .wave-bar');
  const osActiveApp = document.getElementById('osActiveApp');
  const osTrayIcon = document.getElementById('osTrayIcon');
  const osWispTray = document.getElementById('osWispTray');
  const osClock = document.getElementById('osClock');
  const appIcon = document.getElementById('appIcon');
  const appWindowTitle = document.getElementById('appWindowTitle');
  const editorMeta = document.getElementById('editorMeta');
  const appContent = document.getElementById('appContent');
  const holdBtn = document.getElementById('holdBtn');
  const simulateBtn = document.getElementById('simulateBtn');
  const presetChips = document.querySelectorAll('.preset-chip');

  if (!overlay || !appContent) return;

  // Real-world desktop application presets
  const presets = [
    {
      id: 'notes',
      label: '📝 Notes',
      appName: 'Notes',
      appIcon: '📝',
      windowTitle: 'Notes — Sprint Planning',
      meta: '100% Offline (GPU)',
      spoken: 'wisp integrates direct typing with zero clipboard interference and local privacy',
      refined: '- Direct typing: Keystrokes are injected straight into the focused app.\n- 100% Offline: NVIDIA Parakeet and Whisper run locally on-device without cloud telemetry.',
      render: () => `
        <div class="notes-view">
          <div class="notes-doc-title">Sprint 14 Priorities</div>
          <div class="notes-doc-meta">
            <span>📅 Today</span>
            <span>📂 Engineering Sync</span>
            <span>🔒 Local Document</span>
          </div>
          <p class="notes-lead-text">Deliverables for desktop voice injection engine:</p>
          <div class="typed-area">
            <span class="typed-text" id="typedText"></span><span class="caret" aria-hidden="true"></span>
          </div>
        </div>
      `
    },
    {
      id: 'chat',
      label: '💬 Chat',
      appName: 'Slack',
      appIcon: '💬',
      windowTitle: 'Slack — #product-design',
      meta: 'Cloud · 0.8s',
      spoken: 'yeah lets move the product review to thursday afternoon and invite the engineering leads too',
      refined: 'Yeah, let’s move the product review to Thursday afternoon and invite the engineering leads too.',
      render: () => `
        <div class="chat-view">
          <div class="chat-channel-bar">
            <strong>#product-design</strong>
            <span>— Design reviews &amp; sprint alignment</span>
          </div>
          <div class="chat-msg-row">
            <div class="chat-avatar">A</div>
            <div class="chat-msg-content">
              <div class="chat-msg-author">Alex Rivera <span class="chat-msg-time">10:41 PM</span></div>
              <div class="chat-msg-text">Hey team, should we move the product review or keep it on Wednesday morning?</div>
            </div>
          </div>
          <div class="chat-composer-box">
            <div class="chat-composer-placeholder">Message #product-design:</div>
            <div class="typed-area">
              <span class="typed-text" id="typedText"></span><span class="caret" aria-hidden="true"></span>
            </div>
          </div>
        </div>
      `
    },
    {
      id: 'code',
      label: '💻 Code',
      appName: 'Code',
      appIcon: '💻',
      windowTitle: 'VS Code — transcription.py',
      meta: 'Local CPU · Refined',
      spoken: 'define a function called record and transcribe with timeout five seconds and error handling',
      refined: 'def record_and_transcribe(timeout: int = 5) -> str:\n    try:\n        return engine.transcribe(stream_audio(), timeout=timeout)\n    except AudioCaptureError as err:\n        logger.warning(f"Capture error: {err}")\n        return ""',
      render: () => `
        <div class="code-view">
          <div class="code-tabs-bar">
            <span class="code-tab-pill">transcription.py</span>
            <span style="font-size: 11px; color: var(--text-3); padding: 2px 6px;">settings.go</span>
          </div>
          <div class="code-pre-lines">
            <span class="code-kw">import</span> whisper_cpp<br>
            <span class="code-comment"># Global hold-to-record desktop listener</span>
          </div>
          <div class="typed-area">
            <span class="typed-text" id="typedText"></span><span class="caret" aria-hidden="true"></span>
          </div>
        </div>
      `
    },
    {
      id: 'email',
      label: '✉️ Email',
      appName: 'Mail',
      appIcon: '✉️',
      windowTitle: 'Mail — Re: Release Sync',
      meta: 'Cloud · Refined',
      spoken: 'thanks for the update looks great to me go ahead and cut the 1.67 release tomorrow morning',
      refined: 'Hi team,\n\nThanks for the update — looks great to me. Let’s proceed with cutting the v1.67 release tomorrow morning!',
      render: () => `
        <div class="email-view">
          <div class="email-meta-strip">
            <div><strong>To:</strong> founders@acme.ai, eng-leads@acme.ai</div>
            <div><strong>Subject:</strong> Re: Cutting the v1.67 desktop release</div>
          </div>
          <div class="typed-area">
            <span class="typed-text" id="typedText"></span><span class="caret" aria-hidden="true"></span>
          </div>
        </div>
      `
    }
  ];

  let currentPresetIndex = 0;
  let isSimulating = false;
  let isHolding = false;
  let waveAnimInterval = null;
  let timerInterval = null;
  let typingTimeout = null;
  let autoReleaseTimeout = null;

  // Keep OS clock fresh
  if (osClock) {
    const now = new Date();
    osClock.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function setWaveBars(active) {
    if (waveAnimInterval) clearInterval(waveAnimInterval);
    if (!active) {
      waveBars.forEach(bar => {
        bar.style.height = '6px';
        bar.style.opacity = '0.5';
      });
      return;
    }

    waveAnimInterval = setInterval(() => {
      waveBars.forEach((bar, idx) => {
        const base = Math.sin((Date.now() / 140) + idx * 0.45);
        const noise = Math.random() * 0.4;
        const normalized = Math.max(0.15, Math.min(1, Math.abs(base) * 0.7 + noise));
        bar.style.height = `${Math.round(normalized * 22 + 4)}px`;
        bar.style.opacity = '1';
      });
    }, 75);
  }

  function renderPreset(index) {
    const data = presets[index];
    if (osActiveApp) osActiveApp.textContent = data.appName;
    if (appIcon) appIcon.textContent = data.appIcon;
    if (appWindowTitle) appWindowTitle.textContent = data.windowTitle;
    if (editorMeta) editorMeta.textContent = data.meta;
    
    appContent.innerHTML = data.render();
  }

  // Step 1: START RECORDING (Triggered when user holds button or hotkey)
  function startRecording() {
    if (isHolding) return;
    isHolding = true;

    clearTimeout(typingTimeout);
    clearTimeout(autoReleaseTimeout);
    clearInterval(waveAnimInterval);
    clearInterval(timerInterval);

    const typedContainer = document.getElementById('typedText');
    if (typedContainer) typedContainer.textContent = '';

    // HUD enters Recording state
    overlay.className = 'hud-overlay is-recording';
    hudIcon.className = 'hud-mic-indicator recording';
    hudIcon.innerHTML = `<img src="./tray_recording.png" alt="Recording" width="14" height="14">`;
    hudTitle.textContent = 'Listening...';
    hudSub.textContent = 'Hold Ctrl+Super';
    hudTimer.textContent = '00:01';
    setWaveBars(true);

    // Update tray icon in OS bar
    if (osTrayIcon) osTrayIcon.src = './tray_recording.png';
    if (osWispTray) osWispTray.classList.add('recording');

    // Button active state
    if (holdBtn) {
      holdBtn.classList.add('is-holding');
      const label = holdBtn.querySelector('.hold-btn-label');
      if (label) label.textContent = 'Listening...';
    }

    let seconds = 1;
    timerInterval = setInterval(() => {
      seconds++;
      hudTimer.textContent = `00:0${seconds}`;
    }, 1000);
  }

  // Step 2 & 3: FINISH RECORDING -> TRANSCRIBE -> REFINE -> DIRECT INJECT
  function finishRecording() {
    if (!isHolding) return;
    isHolding = false;

    clearInterval(timerInterval);
    clearInterval(waveAnimInterval);
    setWaveBars(false);

    // Reset button
    if (holdBtn) {
      holdBtn.classList.remove('is-holding');
      const label = holdBtn.querySelector('.hold-btn-label');
      if (label) label.textContent = 'Hold to Speak';
    }

    // Reset OS tray
    if (osWispTray) osWispTray.classList.remove('recording');

    const data = presets[currentPresetIndex];
    const typedContainer = document.getElementById('typedText');

    // Stage 1: Transcribing
    overlay.className = 'hud-overlay is-transcribing';
    hudIcon.className = 'hud-mic-indicator transcribing';
    hudIcon.innerHTML = `<img src="./tray_transcribing.png" alt="Transcribing" width="14" height="14">`;
    if (osTrayIcon) osTrayIcon.src = './tray_transcribing.png';
    hudTitle.textContent = 'Transcribing...';
    hudSub.textContent = data.meta.includes('100% Offline') || data.meta.includes('Local') ? 'NVIDIA Parakeet (0.3s)' : 'Groq Whisper Turbo (0.2s)';
    hudTimer.textContent = '0.3s';

    // Stage 2: Refining with LLM
    setTimeout(() => {
      overlay.className = 'hud-overlay is-refined';
      hudIcon.className = 'hud-mic-indicator';
      hudIcon.innerHTML = `<img src="./tray_idle.png" alt="Refining" width="14" height="14">`;
      hudTitle.textContent = 'Refining...';
      hudSub.textContent = 'Polishing grammar & format';
      hudTimer.textContent = '0.5s';

      // Stage 3: Direct Injection into the active app window
      setTimeout(() => {
        overlay.className = 'hud-overlay is-typing';
        hudTitle.textContent = 'Direct Typing';
        hudSub.textContent = `Injecting into ${data.appName}`;
        if (osTrayIcon) osTrayIcon.src = './tray_idle.png';

        if (!typedContainer) return;
        let charIndex = 0;
        const textToType = data.refined;

        function typeNextChar() {
          if (charIndex < textToType.length) {
            typedContainer.textContent += textToType.charAt(charIndex);
            charIndex++;
            const char = textToType.charAt(charIndex - 1);
            const delay = char === '\n' ? 100 : char === ' ' ? 28 : 16;
            typingTimeout = setTimeout(typeNextChar, delay);
          } else {
            // Completed typing!
            setTimeout(() => {
              overlay.className = 'hud-overlay';
              hudTitle.textContent = 'Ready';
              hudSub.textContent = 'Hold Ctrl+Super to speak';
              hudTimer.textContent = '0.4s';
              isSimulating = false;
            }, 1400);
          }
        }
        typeNextChar();

      }, 650);

    }, 700);
  }

  // Trigger full automated simulation
  function runAutomatedSimulation(presetIndex) {
    if (isHolding || isSimulating) {
      clearTimeout(typingTimeout);
      clearTimeout(autoReleaseTimeout);
      finishRecording();
    }
    isSimulating = true;
    currentPresetIndex = presetIndex;
    renderPreset(currentPresetIndex);

    startRecording();

    // After 2.1s of speech, automatically release
    autoReleaseTimeout = setTimeout(() => {
      finishRecording();
    }, 2100);
  }

  // Preset switching
  presetChips.forEach((chip, index) => {
    chip.addEventListener('click', () => {
      presetChips.forEach(c => c.classList.remove('active'));
      chip.classList.add('active');
      currentPresetIndex = index;
      runAutomatedSimulation(currentPresetIndex);
    });
  });

  // Tactile Hold-to-Speak button listeners
  if (holdBtn) {
    // Mouse hold
    holdBtn.addEventListener('mousedown', (e) => {
      e.preventDefault();
      startRecording();
    });

    window.addEventListener('mouseup', () => {
      if (isHolding) finishRecording();
    });

    // Touch hold (mobile / tablet)
    holdBtn.addEventListener('touchstart', (e) => {
      e.preventDefault();
      startRecording();
    });

    holdBtn.addEventListener('touchend', (e) => {
      e.preventDefault();
      if (isHolding) finishRecording();
    });
  }

  // 1-Click Simulate button
  if (simulateBtn) {
    simulateBtn.addEventListener('click', () => {
      runAutomatedSimulation(currentPresetIndex);
    });
  }

  // Initial render
  renderPreset(0);

  // Trigger polite initial demo on load after 900ms
  setTimeout(() => {
    runAutomatedSimulation(0);
  }, 900);
}

/* 3. Interactive Engine & Model Tabs */
function initModelTabs() {
  const tabs = document.querySelectorAll('.model-tab-btn');
  const rows = document.querySelectorAll('.models-table tbody tr');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const filter = tab.dataset.filter;

      rows.forEach(row => {
        const category = row.dataset.category;
        if (filter === 'all' || category === filter) {
          row.style.display = '';
        } else {
          row.style.display = 'none';
        }
      });
    });
  });
}

/* 4. Linux Distro Switcher */
function initDistroSwitcher() {
  const tabs = document.querySelectorAll('.distro-tab');
  const terminalText = document.getElementById('linuxTerminalCommand');
  const copyBtn = document.getElementById('linuxCopyBtn');

  const commands = {
    deb: 'sudo apt install ./wis-free-v3_1.67.4-1_amd64.deb',
    rpm: 'sudo dnf install ./wis-free-v3-1.67.4-1.x86_64.rpm',
    zypper: 'sudo zypper install ./wis-free-v3-1.67.4-1.x86_64.rpm',
    source: 'bash scripts/build-linux.sh --install-user'
  };

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const distro = tab.dataset.distro;
      if (commands[distro] && terminalText && copyBtn) {
        terminalText.textContent = commands[distro];
        copyBtn.dataset.copy = commands[distro];
      }
    });
  });
}

/* 5. 1-Click Clipboard Copy Buttons */
function initCopyButtons() {
  const copyButtons = document.querySelectorAll('.copy-btn');

  copyButtons.forEach(btn => {
    btn.addEventListener('click', async () => {
      const textToCopy = btn.dataset.copy || btn.closest('.terminal-box')?.querySelector('.terminal-code')?.textContent?.trim();
      if (!textToCopy) return;

      try {
        await navigator.clipboard.writeText(textToCopy);
        const originalText = btn.innerHTML;
        btn.classList.add('copied');
        btn.innerHTML = `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg> Copied!`;
        
        setTimeout(() => {
          btn.classList.remove('copied');
          btn.innerHTML = originalText;
        }, 2000);
      } catch (err) {
        console.warn('Clipboard write failed:', err);
      }
    });
  });
}

/* 6. IntersectionObserver Scroll Animations */
function initScrollReveal() {
  const elements = document.querySelectorAll('.reveal');
  if (!elements.length) return;

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('visible');
        observer.unobserve(entry.target);
      }
    });
  }, { threshold: 0.1 });

  elements.forEach((el, index) => {
    el.style.transitionDelay = `${(index % 3) * 70}ms`;
    observer.observe(el);
  });
}

/* 7. Footer Current Year */
function initCurrentYear() {
  const yearElem = document.getElementById('year');
  if (yearElem) {
    yearElem.textContent = new Date().getFullYear();
  }
}
