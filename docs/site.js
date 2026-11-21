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

/* 2. Interactive Desktop Dictation & Floating HUD Overlay Simulator
   Accurately reflects real Wisp overlay.go behavior:
   - Overlay is a compact dark pill (124x44px), HIDDEN by default
   - Recording: overlay appears with 7 animated WHITE waveform bars
   - Transcribing: overlay stays visible, bars pulse in GOLD/ORANGE
   - Injected: overlay DISAPPEARS, and text types directly into the active app window
*/
function initDictationSimulator() {
  const overlay = document.getElementById('hudOverlay');
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

  // Real-world desktop application presets: Notes, Chat (Slack), Email
  const presets = [
    {
      id: 'notes',
      appName: 'Notes',
      appIcon: '📝',
      windowTitle: 'Notes — Sprint Planning',
      meta: '100% Offline (GPU)',
      refined: '- Direct typing: Keystrokes are injected straight into the focused app.\n- 100% offline: NVIDIA Parakeet and Whisper run locally with zero cloud telemetry.',
      render: () => `
        <div class="notes-view">
          <div class="notes-doc-title">Sprint 14 Priorities</div>
          <div class="notes-doc-meta">
            <span>📅 Today</span>
            <span>📂 Engineering Sync</span>
            <span>🔒 Local Note</span>
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
      appName: 'Slack',
      appIcon: '💬',
      windowTitle: 'Slack — #product-design',
      meta: 'Cloud · 0.8s',
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
      id: 'email',
      appName: 'Mail',
      appIcon: '✉️',
      windowTitle: 'Mail — Re: Release Sync',
      meta: 'Cloud · Refined',
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
  let typingTimeout = null;
  let autoReleaseTimeout = null;

  // Keep OS clock accurate
  if (osClock) {
    const updateClock = () => {
      osClock.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };
    updateClock();
    setInterval(updateClock, 30000);
  }

  // Wave bar animation matching overlay.go
  function startWaveBars(mode) {
    if (waveAnimInterval) clearInterval(waveAnimInterval);

    if (mode === 'recording') {
      // overlay.go: t := float64(time.Now().UnixNano()) / 1e9 * 6.0
      waveAnimInterval = setInterval(() => {
        const t = (Date.now() / 1000) * 6.0;
        waveBars.forEach((bar, i) => {
          const phase = i * 0.8;
          const animH = Math.abs(Math.sin(t + phase));
          const h = 8 + animH * 18;
          bar.style.height = `${Math.round(h)}px`;
        });
      }, 50);
    } else if (mode === 'transcribing') {
      // overlay.go: t := float64(time.Now().UnixNano()) / 1e9 * 3.0
      waveAnimInterval = setInterval(() => {
        const t = (Date.now() / 1000) * 3.0;
        waveBars.forEach((bar, i) => {
          const h = 10 + 10 * Math.abs(Math.sin(t + i * 0.3));
          bar.style.height = `${Math.round(h)}px`;
        });
      }, 50);
    }
  }

  function stopWaveBars() {
    if (waveAnimInterval) clearInterval(waveAnimInterval);
    waveAnimInterval = null;
    waveBars.forEach(bar => { bar.style.height = '8px'; });
  }

  function renderPreset(index) {
    const data = presets[index];
    if (osActiveApp) osActiveApp.textContent = data.appName;
    if (appIcon) appIcon.textContent = data.appIcon;
    if (appWindowTitle) appWindowTitle.textContent = data.windowTitle;
    if (editorMeta) editorMeta.textContent = data.meta;
    appContent.innerHTML = data.render();
  }

  // Phase 1: User holds hotkey / button -> Overlay APPEARS with white waveform bars
  function startRecording() {
    if (isHolding) return;
    isHolding = true;

    clearTimeout(typingTimeout);
    clearTimeout(autoReleaseTimeout);
    stopWaveBars();

    const typedContainer = document.getElementById('typedText');
    if (typedContainer) typedContainer.textContent = '';

    // Overlay appears: white animated bars
    overlay.className = 'hud-overlay is-recording';
    if (osTrayIcon) osTrayIcon.src = './tray_recording.png';
    if (osWispTray) osWispTray.classList.add('recording');
    if (holdBtn) {
      holdBtn.classList.add('is-holding');
      const lbl = holdBtn.querySelector('.hold-btn-label');
      if (lbl) lbl.textContent = 'Recording...';
    }
    startWaveBars('recording');
  }

  // Phase 2: User releases hotkey -> Overlay turns orange/gold (Transcribing...)
  // Phase 3: Text is injected into focused app -> Overlay DISAPPEARS completely
  function finishRecording() {
    if (!isHolding) return;
    isHolding = false;

    stopWaveBars();
    if (holdBtn) {
      holdBtn.classList.remove('is-holding');
      const lbl = holdBtn.querySelector('.hold-btn-label');
      if (lbl) lbl.textContent = 'Hold to Speak';
    }
    if (osWispTray) osWispTray.classList.remove('recording');

    const data = presets[currentPresetIndex];
    const typedContainer = document.getElementById('typedText');

    // Overlay turns orange/gold during transcribing
    overlay.className = 'hud-overlay is-transcribing';
    if (osTrayIcon) osTrayIcon.src = './tray_transcribing.png';
    startWaveBars('transcribing');

    // After 600ms processing, overlay disappears and text is injected directly
    setTimeout(() => {
      stopWaveBars();
      // Overlay hides: matches overlay.go Hide()
      overlay.className = 'hud-overlay';
      if (osTrayIcon) osTrayIcon.src = './tray_idle.png';

      if (!typedContainer) return;
      let charIndex = 0;
      const textToType = data.refined;

      function typeNextChar() {
        if (charIndex < textToType.length) {
          typedContainer.textContent += textToType.charAt(charIndex);
          charIndex++;
          const ch = textToType.charAt(charIndex - 1);
          typingTimeout = setTimeout(typeNextChar, ch === '\n' ? 90 : ch === ' ' ? 24 : 14);
        } else {
          isSimulating = false;
        }
      }
      typeNextChar();

    }, 600);
  }

  // Full automated demonstration
  function runAutomatedSimulation(presetIndex) {
    clearTimeout(typingTimeout);
    clearTimeout(autoReleaseTimeout);
    if (isHolding) {
      isHolding = false;
      stopWaveBars();
    }
    isSimulating = true;
    currentPresetIndex = presetIndex;
    renderPreset(presetIndex);

    startRecording();
    autoReleaseTimeout = setTimeout(() => {
      finishRecording();
    }, 2000);
  }

  // Preset switching (Notes, Chat, Email)
  presetChips.forEach((chip, index) => {
    chip.addEventListener('click', () => {
      presetChips.forEach(c => c.classList.remove('active'));
      chip.classList.add('active');
      currentPresetIndex = index;
      runAutomatedSimulation(index);
    });
  });

  // Tactile Hold-to-Speak button
  if (holdBtn) {
    holdBtn.addEventListener('mousedown', e => {
      e.preventDefault();
      startRecording();
    });
    window.addEventListener('mouseup', () => {
      if (isHolding) finishRecording();
    });
    holdBtn.addEventListener('touchstart', e => {
      e.preventDefault();
      startRecording();
    });
    holdBtn.addEventListener('touchend', e => {
      e.preventDefault();
      if (isHolding) finishRecording();
    });
  }

  // Simulate button
  if (simulateBtn) {
    simulateBtn.addEventListener('click', () => {
      runAutomatedSimulation(currentPresetIndex);
    });
  }

  // Initial setup
  renderPreset(0);
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
